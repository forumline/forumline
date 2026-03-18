package realtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/forumline/forumline/services/forumline-api/service"
	"github.com/forumline/forumline/services/forumline-api/store"
)

// PushListener sends web push notifications for new DMs.
// Subscription wiring is done via Watermill Router in main.go.
type PushListener struct {
	Store       *store.Store
	PushService *service.PushService
}

func NewPushListener(s *store.Store, ps *service.PushService) *PushListener {
	return &PushListener{Store: s, PushService: ps}
}

// HandlePayload processes a push_dm event payload and sends web push notifications.
// Returns an error on parse failure so Watermill's retry middleware can re-attempt.
func (pl *PushListener) HandlePayload(ctx context.Context, raw []byte) error {
	var payload struct {
		ConversationID string   `json:"conversation_id"`
		SenderID       string   `json:"sender_id"`
		MemberIDs      []string `json:"member_ids"`
		Content        string   `json:"content"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return fmt.Errorf("parse payload: %w", err)
	}

	senderUsername := pl.Store.GetSenderUsername(ctx, payload.SenderID)
	title := fmt.Sprintf("Message from %s", senderUsername)
	body := payload.Content
	if len(body) > 100 {
		body = body[:100]
	}

	for _, memberID := range payload.MemberIDs {
		if memberID == payload.SenderID {
			continue
		}
		sent := pl.PushService.SendToUser(ctx, memberID, title, body, "", "")
		if sent > 0 {
			log.Printf("PushListener: sent %d push notifications for DM to %s", sent, memberID)
		}
	}
	return nil
}
