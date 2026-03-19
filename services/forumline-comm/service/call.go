package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/livekit/protocol/livekit"
	lksdk "github.com/livekit/server-sdk-go/v2"

	"github.com/forumline/forumline/backend/events"
	"github.com/forumline/forumline/backend/pubsub"
	"github.com/forumline/forumline/services/forumline-comm/store"
)

// LiveKitClient wraps the LiveKit Room Service API for creating/deleting rooms.
type LiveKitClient struct {
	roomClient *lksdk.RoomServiceClient
}

// NewLiveKitClient creates a new LiveKit room service client.
// livekitURL should be the HTTP(S) URL of the LiveKit server.
func NewLiveKitClient(livekitURL, apiKey, apiSecret string) *LiveKitClient {
	httpHost := strings.Replace(strings.Replace(livekitURL, "wss://", "https://", 1), "ws://", "http://", 1)
	return &LiveKitClient{
		roomClient: lksdk.NewRoomServiceClient(httpHost, apiKey, apiSecret),
	}
}

// CreateRoom creates a LiveKit room with the given name and metadata JSON.
func (lk *LiveKitClient) CreateRoom(ctx context.Context, roomName string, metadata string) (*livekit.Room, error) {
	return lk.roomClient.CreateRoom(ctx, &livekit.CreateRoomRequest{
		Name:            roomName,
		EmptyTimeout:    60, // close room if nobody joins within 60s
		DepartureTimeout: 10, // close room 10s after last participant leaves
		MaxParticipants: 2,
		Metadata:        metadata,
	})
}

// DeleteRoom deletes a LiveKit room.
func (lk *LiveKitClient) DeleteRoom(ctx context.Context, roomName string) error {
	_, err := lk.roomClient.DeleteRoom(ctx, &livekit.DeleteRoomRequest{
		Room: roomName,
	})
	return err
}

type CallService struct {
	Store       *store.Store
	PushService *PushService
	EventBus    pubsub.EventBus
	LK          *LiveKitClient
}

func NewCallService(s *store.Store, ps *PushService, bus pubsub.EventBus, lk *LiveKitClient) *CallService {
	return &CallService{Store: s, PushService: ps, EventBus: bus, LK: lk}
}

// RoomMetadata is stored as JSON in the LiveKit room metadata field.
// This lets the webhook handler look up call context without a DB query.
type RoomMetadata struct {
	CallID         string `json:"call_id"`
	ConversationID string `json:"conversation_id"`
	CallerID       string `json:"caller_id"`
	CalleeID       string `json:"callee_id"`
}

type InitiateResult struct {
	Call *store.CallRecord
}

func (cs *CallService) Initiate(ctx context.Context, callerID string, conversationID uuid.UUID) (*InitiateResult, error) {
	calleeID, err := cs.Store.GetCalleeFor1to1(ctx, callerID, conversationID)
	if err != nil {
		return nil, &NotFoundError{Msg: "1:1 conversation not found"}
	}

	// Generate a unique room name: call_{conversationId}_{timestamp}
	roomName := fmt.Sprintf("call_%s_%d", conversationID.String(), time.Now().UnixMilli())

	// Create call record first (for the ID)
	call, err := cs.Store.CreateCallRecord(ctx, conversationID, callerID, calleeID, roomName)
	if err != nil {
		return nil, fmt.Errorf("failed to create call record: %w", err)
	}

	// Create LiveKit room with metadata
	if cs.LK != nil {
		meta := RoomMetadata{
			CallID:         call.ID.String(),
			ConversationID: conversationID.String(),
			CallerID:       callerID,
			CalleeID:       calleeID,
		}
		metaJSON, _ := json.Marshal(meta)
		if _, err := cs.LK.CreateRoom(ctx, roomName, string(metaJSON)); err != nil {
			log.Printf("[Call] Warning: failed to create LiveKit room %s: %v", roomName, err)
			// Don't fail the call — frontend can still attempt to connect and LiveKit
			// auto-creates rooms when the first participant joins with a valid token.
		}
	}

	// Publish incoming_call signal to callee
	callerProfile, err := cs.Store.GetProfile(ctx, callerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get caller info: %w", err)
	}
	displayName := "Unknown"
	callerUsername := ""
	var callerAvatarURL *string
	if callerProfile != nil {
		callerUsername = callerProfile.Username
		displayName = callerProfile.DisplayName
		if displayName == "" {
			displayName = callerUsername
		}
		callerAvatarURL = callerProfile.AvatarURL
	}

	callID := call.ID
	convoID := conversationID

	if cs.EventBus != nil {
		_ = events.Publish(cs.EventBus, ctx, "call_signal", events.CallSignalEvent{
			Type:              "incoming_call",
			CallID:            callID,
			ConversationID:    convoID,
			CallerID:          callerID,
			CallerUsername:    callerUsername,
			CallerDisplayName: displayName,
			CallerAvatarURL:   callerAvatarURL,
			TargetUserID:      calleeID,
		})
	}

	go func() { // #nosec G118 -- push must outlive HTTP request
		title := fmt.Sprintf("Incoming call from %s", displayName)
		sent := cs.PushService.SendToUser(context.Background(), calleeID, title, "Tap to answer", "", "")
		if sent > 0 {
			log.Printf("Call push: sent %d notifications to %s", sent, calleeID)
		}
	}()

	return &InitiateResult{Call: call}, nil
}

type RespondResult struct {
	Status string
}

func (cs *CallService) Respond(ctx context.Context, userID string, callID uuid.UUID, action string) (*RespondResult, error) {
	if action != "accept" && action != "decline" {
		return nil, &ValidationError{Msg: "action must be 'accept' or 'decline'"}
	}

	call, err := cs.Store.GetCallByID(ctx, callID)
	if err != nil {
		return nil, &NotFoundError{Msg: "Call not found"}
	}
	if call.Status != "ringing" {
		return nil, &ConflictError{Msg: "Call is not ringing"}
	}
	if call.CalleeID != userID {
		return nil, &ForbiddenError{Msg: "Only the callee can respond"}
	}

	if action == "accept" {
		// Just publish the signal — frontend connects to LiveKit directly,
		// and participant_joined webhook will activate the call record.
		if cs.EventBus != nil {
			_ = events.Publish(cs.EventBus, ctx, "call_signal", events.CallSignalEvent{
				Type:         "call_accepted",
				CallID:       callID,
				TargetUserID: call.CallerID,
			})
		}
		// Mark as active in DB right away so the token endpoint works
		_ = cs.Store.ActivateCall(ctx, callID)
		return &RespondResult{Status: "accepted"}, nil
	}

	// Decline: delete the LiveKit room and update DB
	if cs.LK != nil && call.RoomName != "" {
		if err := cs.LK.DeleteRoom(ctx, call.RoomName); err != nil {
			log.Printf("[Call] Warning: failed to delete LiveKit room %s on decline: %v", call.RoomName, err)
		}
	}
	_ = cs.Store.EndCallWithoutDuration(ctx, callID, "declined")

	if cs.EventBus != nil {
		_ = events.Publish(cs.EventBus, ctx, "call_signal", events.CallSignalEvent{
			Type:         "call_declined",
			CallID:       callID,
			TargetUserID: call.CallerID,
		})
	}

	return &RespondResult{Status: "declined"}, nil
}

type EndResult struct {
	Status string
}

func (cs *CallService) End(ctx context.Context, userID string, callID uuid.UUID) (*EndResult, error) {
	call, err := cs.Store.GetCallByID(ctx, callID)
	if err != nil {
		return nil, &NotFoundError{Msg: "Call not found"}
	}
	if call.Status != "ringing" && call.Status != "active" {
		return nil, &ConflictError{Msg: "Call is not active"}
	}
	if call.CallerID != userID && call.CalleeID != userID {
		return nil, &ForbiddenError{Msg: "Not a participant"}
	}

	// Delete the LiveKit room — this will also trigger room_finished webhook
	if cs.LK != nil && call.RoomName != "" {
		if err := cs.LK.DeleteRoom(ctx, call.RoomName); err != nil {
			log.Printf("[Call] Warning: failed to delete LiveKit room %s on end: %v", call.RoomName, err)
		}
	}

	// Determine final status
	newStatus := "completed"
	if call.Status == "ringing" {
		if userID == call.CallerID {
			newStatus = "cancelled"
		} else {
			newStatus = "missed"
		}
	}

	_ = cs.Store.EndCallWithoutDuration(ctx, callID, newStatus)

	otherUserID := call.CalleeID
	if userID == call.CalleeID {
		otherUserID = call.CallerID
	}

	if cs.EventBus != nil {
		_ = events.Publish(cs.EventBus, ctx, "call_signal", events.CallSignalEvent{
			Type:         "call_ended",
			CallID:       callID,
			EndedBy:      userID,
			TargetUserID: otherUserID,
		})
	}

	return &EndResult{Status: newStatus}, nil
}

// HandleRoomFinished processes a LiveKit room_finished webhook event.
// It finalizes the call record with duration and publishes call_ended if needed.
func (cs *CallService) HandleRoomFinished(ctx context.Context, roomName string, room *livekit.Room) {
	call, err := cs.Store.GetCallByRoomName(ctx, roomName)
	if err != nil {
		// No active call for this room — could be a forum voice room or already ended.
		return
	}

	// Calculate duration from room creation time
	var durationSec int
	if room != nil && room.CreationTime > 0 {
		durationSec = int(time.Now().Unix() - room.CreationTime)
		if durationSec < 0 {
			durationSec = 0
		}
	}

	status := "completed"
	if call.Status == "ringing" {
		status = "missed" // room closed before callee joined
	}

	if durationSec > 0 && call.Status == "active" {
		_ = cs.Store.EndCallWithDuration(ctx, call.ID, status, durationSec)
	} else {
		_ = cs.Store.EndCallWithoutDuration(ctx, call.ID, status)
	}

	// Publish call_ended to both participants
	for _, targetID := range []string{call.CallerID, call.CalleeID} {
		if cs.EventBus != nil {
			_ = events.Publish(cs.EventBus, ctx, "call_signal", events.CallSignalEvent{
				Type:         "call_ended",
				CallID:       call.ID,
				TargetUserID: targetID,
			})
		}
	}
}

// HandleParticipantJoined processes a LiveKit participant_joined webhook event.
// If the second participant has joined, it activates the call.
func (cs *CallService) HandleParticipantJoined(ctx context.Context, roomName string, numParticipants uint32) {
	if numParticipants < 2 {
		return
	}
	call, err := cs.Store.GetCallByRoomName(ctx, roomName)
	if err != nil {
		return
	}
	if call.Status != "ringing" {
		return // already active
	}
	_ = cs.Store.ActivateCall(ctx, call.ID)
}
