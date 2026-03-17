package store

import (
	"context"

	"github.com/forumline/forumline/forum/model"
	"github.com/forumline/forumline/forum/sqlcdb"
)

// ListChannels returns all chat channels ordered by name.
func (s *Store) ListChannels(ctx context.Context) ([]model.Channel, error) {
	rows, err := s.Q.ListChannels(ctx)
	if err != nil {
		return nil, err
	}
	channels := make([]model.Channel, 0, len(rows))
	for _, r := range rows {
		channels = append(channels, model.Channel{
			ID:          uuidStr(r.ID),
			Name:        r.Name,
			Slug:        r.Slug,
			Description: pgtextPtr(r.Description),
			CreatedAt:   tsStr(r.CreatedAt),
		})
	}
	return channels, nil
}

// GetChannelIDBySlug returns the channel ID for a given slug.
func (s *Store) GetChannelIDBySlug(ctx context.Context, slug string) (string, error) {
	id, err := s.Q.GetChannelIDBySlug(ctx, slug)
	if err != nil {
		return "", err
	}
	return uuidStr(id), nil
}

// ListChatMessages returns chat messages for a channel slug (with author profile).
func (s *Store) ListChatMessages(ctx context.Context, slug string) ([]model.ChatMessage, error) {
	rows, err := s.Q.ListChatMessages(ctx, slug)
	if err != nil {
		return nil, err
	}
	messages := make([]model.ChatMessage, 0, len(rows))
	for _, r := range rows {
		messages = append(messages, chatMessageRowToModel(r))
	}
	return messages, nil
}

// InsertChatMessage inserts a chat message.
func (s *Store) InsertChatMessage(ctx context.Context, channelID, authorID, content string) error {
	return s.Q.InsertChatMessage(ctx, sqlcdb.InsertChatMessageParams{
		ChannelID: pgUUID(channelID),
		AuthorID:  pgUUID(authorID),
		Content:   content,
	})
}
