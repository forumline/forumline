package forum

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/forumline/forumline/forum/oapi"
	"github.com/livekit/protocol/auth"
	"github.com/livekit/protocol/livekit"
	lksdk "github.com/livekit/server-sdk-go/v2"
)

// GetLiveKitToken handles POST /api/livekit -- generates an access token for joining a room.
func (h *Handlers) GetLiveKitToken(ctx context.Context, request oapi.GetLiveKitTokenRequestObject) (oapi.GetLiveKitTokenResponseObject, error) {
	body := request.Body
	// Use the local profile UUID as the LiveKit identity
	userID := ProfileUUIDFromContext(ctx)
	userIDStr := userID.String()

	if h.Config.LiveKit == nil {
		return oapi.GetLiveKitToken500JSONResponse{InternalErrorJSONResponse: oapi.InternalErrorJSONResponse{Error: "LiveKit not configured"}}, nil
	}
	apiKey := h.Config.LiveKit.APIKey
	apiSecret := h.Config.LiveKit.APISecret
	livekitURL := h.Config.LiveKit.URL

	// Remove user from any existing rooms (enforce one room at a time, clean up ghosts)
	httpHost := strings.Replace(strings.Replace(livekitURL, "wss://", "https://", 1), "ws://", "http://", 1)
	roomClient := lksdk.NewRoomServiceClient(httpHost, apiKey, apiSecret)

	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	rooms, err := roomClient.ListRooms(timeoutCtx, &livekit.ListRoomsRequest{})
	if err == nil && rooms != nil {
		for _, room := range rooms.Rooms {
			_, removeErr := roomClient.RemoveParticipant(timeoutCtx, &livekit.RoomParticipantIdentity{
				Room:     room.Name,
				Identity: userIDStr,
			})
			_ = removeErr // Not in this room -- ignore
		}
	}

	// Generate access token
	at := auth.NewAccessToken(apiKey, apiSecret)
	grant := &auth.VideoGrant{
		Room:         body.RoomName,
		RoomJoin:     true,
		CanPublish:   boolPtr(true),
		CanSubscribe: boolPtr(true),
	}
	at.SetVideoGrant(grant).
		SetIdentity(userIDStr).
		SetName(body.ParticipantName).
		SetValidFor(6 * time.Hour)

	token, err := at.ToJWT()
	if err != nil {
		log.Printf("[LiveKit] Failed to generate token: %v", err)
		return oapi.GetLiveKitToken500JSONResponse{InternalErrorJSONResponse: oapi.InternalErrorJSONResponse{Error: "Failed to generate token"}}, nil
	}

	return oapi.GetLiveKitToken200JSONResponse{Token: &token}, nil
}

// GetLiveKitParticipants handles GET /api/livekit -- lists participants.
func (h *Handlers) GetLiveKitParticipants(ctx context.Context, request oapi.GetLiveKitParticipantsRequestObject) (oapi.GetLiveKitParticipantsResponseObject, error) {
	empty := oapi.GetLiveKitParticipants200JSONResponse{}

	if h.Config.LiveKit == nil {
		participants := []oapi.LiveKitParticipant{}
		empty.Participants = &participants
		return empty, nil
	}
	apiKey := h.Config.LiveKit.APIKey
	apiSecret := h.Config.LiveKit.APISecret
	livekitURL := h.Config.LiveKit.URL

	httpHost := strings.Replace(strings.Replace(livekitURL, "wss://", "https://", 1), "ws://", "http://", 1)
	roomClient := lksdk.NewRoomServiceClient(httpHost, apiKey, apiSecret)

	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if request.Params.Room != nil && *request.Params.Room != "" {
		// Single room participants
		resp, err := roomClient.ListParticipants(timeoutCtx, &livekit.ListParticipantsRequest{Room: *request.Params.Room})
		participants := []oapi.LiveKitParticipant{}
		if err == nil {
			for _, p := range resp.Participants {
				name := p.Name
				if name == "" {
					name = p.Identity
				}
				participants = append(participants, oapi.LiveKitParticipant{Identity: p.Identity, Name: name})
			}
		}
		return oapi.GetLiveKitParticipants200JSONResponse{Participants: &participants}, nil
	}

	// All rooms
	roomsResp, err := roomClient.ListRooms(timeoutCtx, &livekit.ListRoomsRequest{})
	if err != nil {
		rooms := map[string]oapi.LiveKitRoomInfo{}
		return oapi.GetLiveKitParticipants200JSONResponse{Rooms: &rooms}, nil
	}

	rooms := make(map[string]oapi.LiveKitRoomInfo)
	for _, room := range roomsResp.Rooms {
		pResp, err := roomClient.ListParticipants(timeoutCtx, &livekit.ListParticipantsRequest{Room: room.Name})
		if err != nil || len(pResp.Participants) == 0 {
			continue
		}
		info := oapi.LiveKitRoomInfo{Count: len(pResp.Participants)}
		for _, p := range pResp.Participants {
			name := p.Name
			if name == "" {
				name = p.Identity
			}
			info.Names = append(info.Names, name)
			info.Identities = append(info.Identities, p.Identity)
		}
		rooms[room.Name] = info
	}
	return oapi.GetLiveKitParticipants200JSONResponse{Rooms: &rooms}, nil
}

func boolPtr(b bool) *bool {
	return &b
}
