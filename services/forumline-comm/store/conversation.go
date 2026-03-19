package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/forumline/forumline/services/forumline-comm/sqlcdb"
)

type Conversation struct {
	ID              uuid.UUID            `json:"id"`
	IsGroup         bool                 `json:"isGroup"`
	Name            *string              `json:"name"`
	Members         []ConversationMember `json:"members"`
	LastMessage     string               `json:"lastMessage"`
	LastMessageTime string               `json:"lastMessageTime"`
	UnreadCount     int                  `json:"unreadCount"`
	LastReadSeq     int64                `json:"lastReadSeq"`
}

type ConversationMember struct {
	ID          string  `json:"id"`
	Username    string  `json:"username"`
	DisplayName string  `json:"displayName"`
	AvatarURL   *string `json:"avatarUrl"`
}

// DirectMessage is the API response shape for DM messages.
// With JetStream, these are constructed from pubsub.ConversationMessage.
type DirectMessage struct {
	ID             string    `json:"id"`
	ConversationID string    `json:"conversation_id"`
	SenderID       string    `json:"sender_id"`
	Content        string    `json:"content"`
	CreatedAt      time.Time `json:"created_at"`
	Sequence       uint64    `json:"sequence"`
}

func (s *Store) ListConversations(ctx context.Context, userID string) ([]Conversation, error) {
	rows, err := s.Q.ListConversations(ctx, userID)
	if err != nil {
		return nil, err
	}

	if len(rows) == 0 {
		return []Conversation{}, nil
	}

	convoIDs := make([]uuid.UUID, len(rows))
	for i, r := range rows {
		convoIDs[i] = r.ID
	}

	membersMap, err := s.fetchConversationMembers(ctx, convoIDs)
	if err != nil {
		return nil, err
	}

	conversations := make([]Conversation, 0, len(rows))
	for _, r := range rows {
		members := membersMap[r.ID]
		if members == nil {
			members = []ConversationMember{}
		}
		conversations = append(conversations, Conversation{
			ID: r.ID, IsGroup: r.IsGroup, Name: r.Name,
			Members: members, LastMessage: r.LastMessage,
			LastMessageTime: r.LastMessageTime.Format(time.RFC3339),
			LastReadSeq:     r.LastReadSeq,
		})
	}
	return conversations, nil
}

func (s *Store) GetConversation(ctx context.Context, userID string, conversationID uuid.UUID) (*Conversation, error) {
	row, err := s.Q.GetConversation(ctx, sqlcdb.GetConversationParams{
		UserID:         userID,
		ConversationID: conversationID,
	})
	if err != nil {
		return nil, err
	}

	membersMap, err := s.fetchConversationMembers(ctx, []uuid.UUID{conversationID})
	if err != nil {
		return nil, err
	}
	members := membersMap[conversationID]
	if members == nil {
		members = []ConversationMember{}
	}
	return &Conversation{
		ID: row.ID, IsGroup: row.IsGroup, Name: row.Name,
		Members: members, LastMessage: row.LastMessage,
		LastMessageTime: row.LastMessageTime.Format(time.RFC3339),
		LastReadSeq:     row.LastReadSeq,
	}, nil
}

func (s *Store) fetchConversationMembers(ctx context.Context, convoIDs []uuid.UUID) (map[uuid.UUID][]ConversationMember, error) {
	rows, err := s.Q.FetchConversationMembers(ctx, convoIDs)
	if err != nil {
		return nil, err
	}

	result := make(map[uuid.UUID][]ConversationMember)
	for _, r := range rows {
		name := r.DisplayName
		if name == "" {
			name = r.Username
		}
		result[r.ConversationID] = append(result[r.ConversationID], ConversationMember{
			ID: r.UserID, Username: r.Username, DisplayName: name, AvatarURL: r.AvatarUrl,
		})
	}
	return result, nil
}

func (s *Store) IsConversationMember(ctx context.Context, conversationID uuid.UUID, userID string) (bool, error) {
	return s.Q.IsConversationMember(ctx, sqlcdb.IsConversationMemberParams{
		ConversationID: conversationID,
		UserID:         userID,
	})
}

// TouchConversationWithMessage updates the denormalized last-message columns.
func (s *Store) TouchConversationWithMessage(ctx context.Context, conversationID uuid.UUID, senderID, content string, messageAt time.Time) error {
	return s.Q.TouchConversationWithMessage(ctx, sqlcdb.TouchConversationWithMessageParams{
		Content:        &content,
		SenderID:       &senderID,
		MessageAt:      &messageAt,
		ConversationID: conversationID,
	})
}

// MarkReadSeq stores the JetStream sequence number of the last read message.
func (s *Store) MarkReadSeq(ctx context.Context, conversationID uuid.UUID, userID string, seq int64) error {
	return s.Q.MarkReadSeq(ctx, sqlcdb.MarkReadSeqParams{
		LastReadSeq:    pgtype.Int8{Int64: seq, Valid: true},
		ConversationID: conversationID,
		UserID:         userID,
	})
}

// GetMemberLastReadSeq returns the last_read_seq for a member.
func (s *Store) GetMemberLastReadSeq(ctx context.Context, conversationID uuid.UUID, userID string) (int64, error) {
	return s.Q.GetMemberLastReadSeq(ctx, sqlcdb.GetMemberLastReadSeqParams{
		ConversationID: conversationID,
		UserID:         userID,
	})
}

func (s *Store) FindOrCreate1to1Conversation(ctx context.Context, userID, otherUserID string) (uuid.UUID, error) {
	id, err := s.Q.Find1to1Conversation(ctx, sqlcdb.Find1to1ConversationParams{
		UserID:      userID,
		OtherUserID: otherUserID,
	})
	if err == nil {
		return id, nil
	}

	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return uuid.UUID{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	qtx := s.Q.WithTx(tx)

	convoID, err := qtx.CreateConversation(ctx, sqlcdb.CreateConversationParams{
		IsGroup:   false,
		CreatedBy: &userID,
	})
	if err != nil {
		return uuid.UUID{}, err
	}

	err = qtx.Insert1to1Members(ctx, sqlcdb.Insert1to1MembersParams{
		ConversationID: convoID,
		UserID:         userID,
		UserID_2:       otherUserID,
	})
	if err != nil {
		return uuid.UUID{}, err
	}

	if err = tx.Commit(ctx); err != nil {
		return uuid.UUID{}, err
	}
	return convoID, nil
}

func (s *Store) CreateGroupConversation(ctx context.Context, name, creatorID string, memberIDs []string) (uuid.UUID, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return uuid.UUID{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	qtx := s.Q.WithTx(tx)

	convoID, err := qtx.CreateConversation(ctx, sqlcdb.CreateConversationParams{
		IsGroup:   true,
		Name:      &name,
		CreatedBy: &creatorID,
	})
	if err != nil {
		return uuid.UUID{}, err
	}

	valueStrings := make([]string, len(memberIDs))
	args := make([]interface{}, 0, len(memberIDs)+1)
	args = append(args, convoID)
	for i, id := range memberIDs {
		valueStrings[i] = fmt.Sprintf("($1, $%d)", i+2)
		args = append(args, id)
	}
	_, err = tx.Exec(ctx,
		fmt.Sprintf(`INSERT INTO forumline_conversation_members (conversation_id, user_id) VALUES %s`,
			strings.Join(valueStrings, ",")),
		args...,
	)
	if err != nil {
		return uuid.UUID{}, err
	}

	if err = tx.Commit(ctx); err != nil {
		return uuid.UUID{}, err
	}
	return convoID, nil
}

func (s *Store) IsGroupConversation(ctx context.Context, conversationID uuid.UUID, userID string) (bool, error) {
	return s.Q.IsGroupConversation(ctx, sqlcdb.IsGroupConversationParams{
		UserID:         userID,
		ConversationID: conversationID,
	})
}

func (s *Store) UpdateConversationName(ctx context.Context, conversationID uuid.UUID, name string) error {
	return s.Q.UpdateConversationName(ctx, sqlcdb.UpdateConversationNameParams{
		Name: &name,
		ID:   conversationID,
	})
}

func (s *Store) AddConversationMembers(ctx context.Context, conversationID uuid.UUID, memberIDs []string) error {
	return s.Q.AddConversationMembers(ctx, sqlcdb.AddConversationMembersParams{
		ConversationID: conversationID,
		MemberIds:      memberIDs,
	})
}

func (s *Store) RemoveConversationMembers(ctx context.Context, conversationID uuid.UUID, memberIDs []string) error {
	return s.Q.RemoveConversationMembers(ctx, sqlcdb.RemoveConversationMembersParams{
		ConversationID: conversationID,
		MemberIds:      memberIDs,
	})
}

func (s *Store) GetConversationMemberIDs(ctx context.Context, conversationID uuid.UUID) ([]string, error) {
	return s.Q.GetConversationMemberIDs(ctx, conversationID)
}

func (s *Store) LeaveConversation(ctx context.Context, conversationID uuid.UUID, userID string) error {
	return s.Q.LeaveConversation(ctx, sqlcdb.LeaveConversationParams{
		ConversationID: conversationID,
		UserID:         userID,
	})
}
