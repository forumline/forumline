package store

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/forumline/forumline/forum/oapi"
	"github.com/forumline/forumline/forum/sqlcdb"
)

// GetProfile returns a profile by ID.
func (s *Store) GetProfile(ctx context.Context, id string) (*oapi.Profile, error) {
	row, err := s.Q.GetProfile(ctx, pgUUID(id))
	if err != nil {
		return nil, err
	}
	p := profileFromSqlc(row)
	return &p, nil
}

// GetProfileByUsername returns a profile by username.
func (s *Store) GetProfileByUsername(ctx context.Context, username string) (*oapi.Profile, error) {
	row, err := s.Q.GetProfileByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	p := profileFromSqlc(row)
	return &p, nil
}

// GetProfilesByIDs returns profiles for the given IDs.
func (s *Store) GetProfilesByIDs(ctx context.Context, ids []string) ([]oapi.Profile, error) {
	pgIDs := make([]pgtype.UUID, len(ids))
	for i, id := range ids {
		pgIDs[i] = pgUUID(id)
	}
	rows, err := s.Q.GetProfilesByIDs(ctx, pgIDs)
	if err != nil {
		return nil, err
	}
	profiles := make([]oapi.Profile, 0, len(rows))
	for _, row := range rows {
		profiles = append(profiles, profileFromSqlc(row))
	}
	return profiles, nil
}

// GetProfileByForumlineID returns a profile by forumline_id.
func (s *Store) GetProfileByForumlineID(ctx context.Context, forumlineID string) (*oapi.Profile, error) {
	id, err := s.Q.GetProfileIDByForumlineID(ctx, textToPgtext(forumlineID))
	if err != nil {
		return nil, err
	}
	return s.GetProfile(ctx, uuidStr(id))
}

// GetProfileIDByForumlineID returns just the profile ID for a forumline_id.
func (s *Store) GetProfileIDByForumlineID(ctx context.Context, forumlineID string) (string, error) {
	id, err := s.Q.GetProfileIDByForumlineID(ctx, textToPgtext(forumlineID))
	if err != nil {
		return "", err
	}
	return uuidStr(id), nil
}

// GetProfileIDByForumlineIDUnlinked returns the profile ID if forumline_id is null/empty.
func (s *Store) GetProfileIDByForumlineIDUnlinked(ctx context.Context, userID string) (string, error) {
	id, err := s.Q.GetProfileIDByForumlineIDUnlinked(ctx, pgUUID(userID))
	if err != nil {
		return "", err
	}
	return uuidStr(id), nil
}

// UsernameExists checks if a username is taken.
func (s *Store) UsernameExists(ctx context.Context, username string) (bool, error) {
	return s.Q.UsernameExists(ctx, username)
}

// UpsertProfileFull creates or updates a profile with username.
func (s *Store) UpsertProfileFull(ctx context.Context, id, username string, displayName, avatarURL *string) error {
	now := time.Now()
	return s.Q.UpsertProfileFull(ctx, sqlcdb.UpsertProfileFullParams{
		ID:          pgUUID(id),
		Username:    username,
		DisplayName: optTextToPgtext(displayName),
		AvatarUrl:   optTextToPgtext(avatarURL),
		CreatedAt:   pgTimestamp(now),
	})
}

// UpdateProfilePartial performs a dynamic partial update on a profile.
func (s *Store) UpdateProfilePartial(ctx context.Context, userID string, setClauses []string, args []interface{}) error {
	query := "UPDATE profiles SET "
	for i, clause := range setClauses {
		if i > 0 {
			query += ", "
		}
		query += clause
	}
	query += " WHERE id = $1"
	_, err := s.DB.Exec(ctx, query, args...)
	return err
}

// ClearForumlineID removes the forumline_id from a profile.
func (s *Store) ClearForumlineID(ctx context.Context, userID string) error {
	return s.Q.ClearForumlineID(ctx, pgUUID(userID))
}

// SetForumlineID sets the forumline_id on a profile.
func (s *Store) SetForumlineID(ctx context.Context, userID, forumlineID string) error {
	return s.Q.SetForumlineID(ctx, sqlcdb.SetForumlineIDParams{
		ForumlineID: textToPgtext(forumlineID),
		ID:          pgUUID(userID),
	})
}

// EnsureProfileWithForumlineID creates or updates a profile with the forumline_id set.
func (s *Store) EnsureProfileWithForumlineID(ctx context.Context, userID string, identity *ForumlineIdentity) error {
	return s.Q.EnsureProfileWithForumlineID(ctx, sqlcdb.EnsureProfileWithForumlineIDParams{
		ID:          pgUUID(userID),
		Username:    identity.Username,
		DisplayName: textToPgtext(identity.DisplayName),
		ForumlineID: textToPgtext(identity.ForumlineID),
	})
}

// GetForumlineID returns the forumline_id for a user.
func (s *Store) GetForumlineID(ctx context.Context, userID string) (*string, error) {
	t, err := s.Q.GetForumlineID(ctx, pgUUID(userID))
	if err != nil {
		return nil, err
	}
	return pgtextPtr(t), nil
}

// IsAdmin returns whether a user is an admin.
func (s *Store) IsAdmin(ctx context.Context, userID string) (bool, error) {
	return s.Q.IsAdmin(ctx, pgUUID(userID))
}

// ListProfiles returns profiles ordered by created_at desc.
func (s *Store) ListProfiles(ctx context.Context, limit int) ([]oapi.Profile, error) {
	rows, err := s.Q.ListProfiles(ctx, int32(min(limit, 1000))) //nolint:gosec // bounded
	if err != nil {
		return nil, err
	}
	profiles := make([]oapi.Profile, 0, len(rows))
	for _, row := range rows {
		profiles = append(profiles, profileFromSqlc(row))
	}
	return profiles, nil
}

// GetUserIDByUsername returns a user ID for a username (case-insensitive).
func (s *Store) GetUserIDByUsername(ctx context.Context, username string) (string, error) {
	id, err := s.Q.GetUserIDByUsername(ctx, username)
	if err != nil {
		return "", err
	}
	return uuidStr(id), nil
}

// GetUsername returns the username for a user ID.
func (s *Store) GetUsername(ctx context.Context, userID string) (string, error) {
	return s.Q.GetUsername(ctx, pgUUID(userID))
}

// UpdateDisplayName updates a profile's display_name.
func (s *Store) UpdateDisplayName(ctx context.Context, userID, displayName string) error {
	return s.Q.UpdateDisplayName(ctx, sqlcdb.UpdateDisplayNameParams{
		DisplayName: textToPgtext(displayName),
		ID:          pgUUID(userID),
	})
}

// UpdateDisplayNameAndAvatar updates display_name and avatar_url.
func (s *Store) UpdateDisplayNameAndAvatar(ctx context.Context, userID, displayName, avatarURL string) error {
	return s.Q.UpdateDisplayNameAndAvatar(ctx, sqlcdb.UpdateDisplayNameAndAvatarParams{
		DisplayName: textToPgtext(displayName),
		AvatarUrl:   textToPgtext(avatarURL),
		ID:          pgUUID(userID),
	})
}

// CreateProfileSimple creates a basic profile with username.
func (s *Store) CreateProfileSimple(ctx context.Context, id, username, displayName string) error {
	return s.Q.CreateProfileSimple(ctx, sqlcdb.CreateProfileSimpleParams{
		ID:          pgUUID(id),
		Username:    username,
		DisplayName: textToPgtext(displayName),
	})
}

// CreateProfileHosted creates a profile for hosted mode with a generated UUID
// and stores the forumline_id for identity linking.
func (s *Store) CreateProfileHosted(ctx context.Context, identity *ForumlineIdentity) (string, error) {
	id := uuid.New().String()
	err := s.Q.CreateProfileHosted(ctx, sqlcdb.CreateProfileHostedParams{
		ID:          pgUUID(id),
		Username:    identity.Username,
		DisplayName: textToPgtext(identity.DisplayName),
		AvatarUrl:   textToPgtext(identity.AvatarURL),
		ForumlineID: textToPgtext(identity.ForumlineID),
	})
	return id, err
}

// ForumlineIdentity represents identity data from the Forumline auth service,
// used when creating or syncing profiles in hosted mode.
type ForumlineIdentity struct {
	ForumlineID string
	Username    string
	DisplayName string
	AvatarURL   string
	Bio         string
}
