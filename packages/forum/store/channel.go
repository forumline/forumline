package store

import (
	"context"

	"github.com/google/uuid"

	"github.com/forumline/forumline/forum/oapi"
	"github.com/forumline/forumline/forum/sqlcdb"
)

// ListChannels returns all chat channels ordered by name.
func (s *Store) ListChannels(ctx context.Context) ([]oapi.Channel, error) {
	rows, err := s.Q.ListChannels(ctx)
	if err != nil {
		return nil, err
	}
	channels := make([]oapi.Channel, 0, len(rows))
	for _, r := range rows {
		channels = append(channels, oapi.Channel{
			Id:          r.ID,
			Name:        r.Name,
			Slug:        r.Slug,
			Description: pgtextPtr(r.Description),
			CreatedAt:   tsTime(r.CreatedAt),
		})
	}
	return channels, nil
}

// GetChannelIDBySlug returns the channel UUID for a given slug.
func (s *Store) GetChannelIDBySlug(ctx context.Context, slug string) (uuid.UUID, error) {
	return s.Q.GetChannelIDBySlug(ctx, slug)
}

// ListChatMessages returns chat messages for a channel slug (with author profile).
func (s *Store) ListChatMessages(ctx context.Context, slug string) ([]oapi.ChatMessage, error) {
	rows, err := s.Q.ListChatMessages(ctx, slug)
	if err != nil {
		return nil, err
	}
	messages := make([]oapi.ChatMessage, 0, len(rows))
	for _, r := range rows {
		messages = append(messages, chatMessageRowToOapi(r))
	}
	return messages, nil
}

// InsertChatMessage inserts a chat message.
func (s *Store) InsertChatMessage(ctx context.Context, channelID, authorID uuid.UUID, content string) error {
	return s.Q.InsertChatMessage(ctx, sqlcdb.InsertChatMessageParams{
		ChannelID: channelID,
		AuthorID:  authorID,
		Content:   content,
	})
}
