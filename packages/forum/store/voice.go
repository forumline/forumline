package store

import (
	"context"
	"time"

	"github.com/forumline/forumline/forum/model"
	"github.com/forumline/forumline/forum/sqlcdb"
)

// ListVoiceRooms returns all voice rooms ordered by name.
func (s *Store) ListVoiceRooms(ctx context.Context) ([]model.VoiceRoom, error) {
	rows, err := s.Q.ListVoiceRooms(ctx)
	if err != nil {
		return nil, err
	}
	rooms := make([]model.VoiceRoom, 0, len(rows))
	for _, r := range rows {
		rooms = append(rooms, model.VoiceRoom{
			ID:        uuidStr(r.ID),
			Name:      r.Name,
			Slug:      r.Slug,
			CreatedAt: tsStr(r.CreatedAt),
		})
	}
	return rooms, nil
}

// ListVoicePresence returns all voice presence entries with profile.
func (s *Store) ListVoicePresence(ctx context.Context) ([]model.VoicePresence, error) {
	rows, err := s.Q.ListVoicePresence(ctx)
	if err != nil {
		return nil, err
	}
	presence := make([]model.VoicePresence, 0, len(rows))
	for _, r := range rows {
		presence = append(presence, voicePresenceRowToModel(r))
	}
	return presence, nil
}

// SetVoicePresence sets a user's voice presence.
func (s *Store) SetVoicePresence(ctx context.Context, userID, roomSlug string) error {
	now := time.Now()
	return s.Q.SetVoicePresence(ctx, sqlcdb.SetVoicePresenceParams{
		UserID:   pgUUID(userID),
		RoomSlug: roomSlug,
		JoinedAt: pgTimestamp(now),
	})
}

// ClearVoicePresence removes a user's voice presence.
func (s *Store) ClearVoicePresence(ctx context.Context, userID string) error {
	return s.Q.ClearVoicePresence(ctx, pgUUID(userID))
}
