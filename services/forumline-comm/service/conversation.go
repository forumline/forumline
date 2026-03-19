package service

import (
	"context"
	"fmt"
	"log"
	"math"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/forumline/forumline/backend/events"
	"github.com/forumline/forumline/backend/pubsub"
	"github.com/forumline/forumline/services/forumline-comm/store"
)

type ConversationService struct {
	Store    *store.Store
	EventBus pubsub.EventBus
	JSM      *pubsub.JetStreamManager
}

func NewConversationService(s *store.Store, bus pubsub.EventBus, jsm *pubsub.JetStreamManager) *ConversationService {
	return &ConversationService{Store: s, EventBus: bus, JSM: jsm}
}

func (cs *ConversationService) GetOrCreateDM(ctx context.Context, userID, otherUserID string) (uuid.UUID, error) {
	if otherUserID == "" {
		return uuid.UUID{}, &ValidationError{Msg: "userId is required"}
	}
	if otherUserID == userID {
		return uuid.UUID{}, &ValidationError{Msg: "cannot message yourself"}
	}
	exists, err := cs.Store.ProfileExists(ctx, otherUserID)
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("failed to verify user: %w", err)
	}
	if !exists {
		return uuid.UUID{}, &NotFoundError{Msg: "user not found"}
	}
	convoID, err := cs.Store.FindOrCreate1to1Conversation(ctx, userID, otherUserID)
	if err != nil {
		return uuid.UUID{}, err
	}

	// Ensure JetStream stream exists for this conversation.
	if err := cs.JSM.EnsureConversationStream(convoID.String()); err != nil {
		log.Printf("[dm] failed to ensure JetStream stream for %s: %v", convoID, err)
	}

	return convoID, nil
}

type CreateGroupInput struct {
	Name      string
	MemberIDs []string
}

func (cs *ConversationService) CreateGroup(ctx context.Context, userID string, input CreateGroupInput) (*store.Conversation, error) {
	if len(input.MemberIDs) < 2 {
		return nil, &ValidationError{Msg: "group must have at least 2 other members"}
	}
	if len(input.MemberIDs) > 50 {
		return nil, &ValidationError{Msg: "group cannot exceed 50 members"}
	}
	if input.Name == "" || len(input.Name) > 100 {
		return nil, &ValidationError{Msg: "group name must be 1-100 characters"}
	}

	allMembers := deduplicateMembers(userID, input.MemberIDs)
	if len(allMembers) < 3 {
		return nil, &ValidationError{Msg: "group must have at least 3 members including yourself"}
	}

	count, _ := cs.Store.CountExistingUsers(ctx, allMembers)
	if count != len(allMembers) {
		return nil, &ValidationError{Msg: "one or more users not found"}
	}

	convoID, err := cs.Store.CreateGroupConversation(ctx, input.Name, userID, allMembers)
	if err != nil {
		return nil, fmt.Errorf("failed to create group: %w", err)
	}

	// Ensure JetStream stream exists for this conversation.
	if err := cs.JSM.EnsureConversationStream(convoID.String()); err != nil {
		log.Printf("[dm] failed to ensure JetStream stream for group %s: %v", convoID, err)
	}

	profiles, _ := cs.Store.FetchProfilesByIDs(ctx, allMembers)
	members := buildMemberList(allMembers, profiles)

	return &store.Conversation{
		ID:      convoID,
		IsGroup: true,
		Name:    &input.Name,
		Members: members,
	}, nil
}

type UpdateInput struct {
	Name          *string
	AddMembers    []string
	RemoveMembers []string
}

func (cs *ConversationService) Update(ctx context.Context, userID string, conversationID uuid.UUID, input UpdateInput) error {
	isGroup, err := cs.Store.IsGroupConversation(ctx, conversationID, userID)
	if err != nil {
		return &NotFoundError{Msg: "conversation not found"}
	}
	if !isGroup {
		return &ValidationError{Msg: "cannot modify a 1:1 conversation"}
	}

	if input.Name != nil {
		name := trimString(*input.Name)
		if name == "" || len(name) > 100 {
			return &ValidationError{Msg: "group name must be 1-100 characters"}
		}
		if err := cs.Store.UpdateConversationName(ctx, conversationID, name); err != nil {
			return fmt.Errorf("failed to update name: %w", err)
		}
	}
	if len(input.AddMembers) > 0 {
		if err := cs.Store.AddConversationMembers(ctx, conversationID, input.AddMembers); err != nil {
			return fmt.Errorf("failed to add members: %w", err)
		}
	}
	if len(input.RemoveMembers) > 0 {
		var filtered []string
		for _, id := range input.RemoveMembers {
			if id != userID {
				filtered = append(filtered, id)
			}
		}
		if len(filtered) > 0 {
			if err := cs.Store.RemoveConversationMembers(ctx, conversationID, filtered); err != nil {
				return fmt.Errorf("failed to remove members: %w", err)
			}
		}
	}
	return nil
}

func (cs *ConversationService) GetMessages(ctx context.Context, userID string, conversationID uuid.UUID, before, limitStr string) ([]store.DirectMessage, error) {
	isMember, _ := cs.Store.IsConversationMember(ctx, conversationID, userID)
	if !isMember {
		return nil, &NotFoundError{Msg: "conversation not found"}
	}

	limit := 50
	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
		limit = int(math.Min(float64(l), 100))
	}

	var beforeSeq uint64
	if before != "" {
		parsed, err := strconv.ParseUint(before, 10, 64)
		if err != nil {
			return nil, &ValidationError{Msg: "invalid before cursor"}
		}
		beforeSeq = parsed
	}

	msgs, err := cs.JSM.GetMessages(ctx, conversationID.String(), limit, beforeSeq)
	if err != nil {
		return nil, fmt.Errorf("failed to get messages: %w", err)
	}

	result := make([]store.DirectMessage, len(msgs))
	for i, m := range msgs {
		result[i] = store.DirectMessage{
			ID:             m.ID,
			ConversationID: conversationID.String(),
			SenderID:       m.SenderID,
			Content:        m.Content,
			CreatedAt:      m.CreatedAt,
			Sequence:       m.Sequence,
		}
	}
	return result, nil
}

func (cs *ConversationService) SendMessage(ctx context.Context, userID string, conversationID uuid.UUID, content string) (*store.DirectMessage, error) {
	isMember, _ := cs.Store.IsConversationMember(ctx, conversationID, userID)
	if !isMember {
		return nil, &NotFoundError{Msg: "conversation not found"}
	}
	content = trimString(content)
	if content == "" || len(content) > 2000 {
		return nil, &ValidationError{Msg: "message must be 1-2000 characters"}
	}

	now := time.Now()
	msgID := uuid.New().String()

	jsMsg := &pubsub.ConversationMessage{
		ID:        msgID,
		SenderID:  userID,
		Content:   content,
		CreatedAt: now,
	}

	seq, err := cs.JSM.PublishMessage(conversationID.String(), jsMsg)
	if err != nil {
		return nil, fmt.Errorf("failed to publish message: %w", err)
	}

	// Update denormalized last-message columns in Postgres.
	if err := cs.Store.TouchConversationWithMessage(ctx, conversationID, userID, content, now); err != nil {
		log.Printf("[dm] failed to touch conversation %s: %v", conversationID, err)
	}

	msg := &store.DirectMessage{
		ID:             msgID,
		ConversationID: conversationID.String(),
		SenderID:       userID,
		Content:        content,
		CreatedAt:      now,
		Sequence:       seq,
	}

	// Fire SSE events via the fire-and-forget Watermill bus.
	if cs.EventBus != nil {
		memberIDs, _ := cs.Store.GetConversationMemberIDs(ctx, conversationID)
		if pubErr := events.Publish(cs.EventBus, ctx, "dm_changes", events.DmEvent{
			ConversationID: conversationID,
			SenderID:       msg.SenderID,
			MemberIDs:      memberIDs,
			ID:             uuid.MustParse(msg.ID),
			Content:        msg.Content,
			CreatedAt:      msg.CreatedAt.Format(time.RFC3339Nano),
		}); pubErr != nil {
			log.Printf("[dm] EventBus publish error: %v", pubErr)
		}

		if pubErr := events.Publish(cs.EventBus, ctx, "push_dm", events.PushDmEvent{
			ConversationID: conversationID,
			SenderID:       msg.SenderID,
			MemberIDs:      memberIDs,
			Content:        msg.Content,
		}); pubErr != nil {
			log.Printf("[dm] EventBus push publish error: %v", pubErr)
		}
	}

	return msg, nil
}

func (cs *ConversationService) Leave(ctx context.Context, userID string, conversationID uuid.UUID) error {
	isGroup, err := cs.Store.IsGroupConversation(ctx, conversationID, userID)
	if err != nil {
		return &NotFoundError{Msg: "conversation not found"}
	}
	if !isGroup {
		return &ValidationError{Msg: "cannot leave a 1:1 conversation"}
	}
	return cs.Store.LeaveConversation(ctx, conversationID, userID)
}

func (cs *ConversationService) ResolveConversationID(ctx context.Context, userID, otherUserID string) (uuid.UUID, error) {
	if otherUserID == "" || otherUserID == userID {
		return uuid.UUID{}, &NotFoundError{Msg: "conversation not found"}
	}
	return cs.Store.FindOrCreate1to1Conversation(ctx, userID, otherUserID)
}

func deduplicateMembers(creatorID string, memberIDs []string) []string {
	seen := map[string]bool{creatorID: true}
	unique := []string{creatorID}
	for _, id := range memberIDs {
		if id != "" && !seen[id] {
			seen[id] = true
			unique = append(unique, id)
		}
	}
	return unique
}

func buildMemberList(ids []string, profiles map[string]*store.Profile) []store.ConversationMember {
	members := make([]store.ConversationMember, 0, len(ids))
	for _, id := range ids {
		m := store.ConversationMember{ID: id}
		if p := profiles[id]; p != nil {
			m.Username = p.Username
			m.DisplayName = p.DisplayName
			if m.DisplayName == "" {
				m.DisplayName = p.Username
			}
			m.AvatarURL = p.AvatarURL
		}
		members = append(members, m)
	}
	return members
}
