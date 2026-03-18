// Package events defines typed event structs for all realtime SSE events.
//
// These structs are the single source of truth for event payloads published
// to NATS and consumed by the SSE hub. They replace the hand-rolled
// map[string]interface{} payloads that previously required manual field
// synchronization between Go publishers and TypeScript consumers.
//
// The TypeScript equivalents are generated from the OpenAPI spec
// (services/forumline-api/openapi.yaml) — keep both in sync.
package events

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/forumline/forumline/backend/pubsub"
)

// --- Forumline App Events (forumline-api) ---

// DmEvent is published to "dm_changes" when a new DM is sent.
type DmEvent struct {
	ConversationID uuid.UUID `json:"conversation_id"`
	SenderID       string    `json:"sender_id"`
	MemberIDs      []string  `json:"member_ids"`
	ID             uuid.UUID `json:"id"`
	Content        string    `json:"content"`
	CreatedAt      string    `json:"created_at"`
}

// PushDmEvent is published to "push_dm" to trigger web push notifications.
type PushDmEvent struct {
	ConversationID uuid.UUID `json:"conversation_id"`
	SenderID       string    `json:"sender_id"`
	MemberIDs      []string  `json:"member_ids"`
	Content        string    `json:"content"`
}

// CallSignalEvent is published to "call_signal" for call signaling.
type CallSignalEvent struct {
	Type              string    `json:"type"` // incoming_call, call_accepted, call_declined, call_ended
	CallID            uuid.UUID `json:"call_id"`
	ConversationID    uuid.UUID `json:"conversation_id,omitempty"`
	CallerID          string    `json:"caller_id,omitempty"`
	CallerUsername    string    `json:"caller_username,omitempty"`
	CallerDisplayName string    `json:"caller_display_name,omitempty"`
	CallerAvatarURL   *string   `json:"caller_avatar_url,omitempty"`
	EndedBy           string    `json:"ended_by,omitempty"`
	TargetUserID      string    `json:"target_user_id"`
}

// ForumlineNotificationEvent is published to "forumline_notification_changes"
// when a forum sends a notification to the Forumline hub.
type ForumlineNotificationEvent struct {
	ID          uuid.UUID `json:"id"`
	UserID      string    `json:"user_id"`
	ForumDomain string    `json:"forum_domain"`
	ForumName   string    `json:"forum_name"`
	Type        string    `json:"type"` // reply, mention, chat_mention, dm, custom
	Title       string    `json:"title"`
	Body        string    `json:"body"`
	Link        string    `json:"link"`
	Read        bool      `json:"read"`
	CreatedAt   time.Time `json:"created_at"`
}

// --- Forum Engine Events (packages/forum) ---

// ChatMessageEvent is published to "chat_message_changes" for new chat messages.
type ChatMessageEvent struct {
	Schema    string    `json:"schema"`
	ID        uuid.UUID `json:"id"`
	ChannelID uuid.UUID `json:"channel_id"`
	AuthorID  uuid.UUID `json:"author_id"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// PostEvent is published to "post_changes" for new forum posts.
type PostEvent struct {
	Schema    string     `json:"schema"`
	ID        uuid.UUID  `json:"id"`
	ThreadID  uuid.UUID  `json:"thread_id"`
	AuthorID  uuid.UUID  `json:"author_id"`
	Content   string     `json:"content"`
	ReplyToID *uuid.UUID `json:"reply_to_id"`
	CreatedAt time.Time  `json:"created_at"`
}

// NotificationChangeEvent is published to "notification_changes" for
// per-forum notifications (not the cross-forum forumline_notification_changes).
type NotificationChangeEvent struct {
	Schema    string    `json:"schema"`
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	Type      string    `json:"type"` // reply, mention
	Title     string    `json:"title"`
	Message   string    `json:"message"`
	Link      string    `json:"link"`
	Read      bool      `json:"read"`
	CreatedAt time.Time `json:"created_at"`
}

// VoicePresenceEvent is published to "voice_presence_changes" when a user
// joins or leaves a voice room.
type VoicePresenceEvent struct {
	Schema   string    `json:"schema"`
	Event    string    `json:"event"` // INSERT or DELETE
	UserID   uuid.UUID `json:"user_id"`
	RoomSlug string    `json:"room_slug"`
	JoinedAt time.Time `json:"joined_at,omitempty"`
}

// Publish marshals a typed event to JSON and publishes it to the given topic.
func Publish[T any](bus pubsub.EventBus, ctx context.Context, topic string, event T) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal %s event: %w", topic, err)
	}
	return bus.Publish(ctx, topic, data)
}
