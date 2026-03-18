package store

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/forumline/forumline/forum/oapi"
	"github.com/forumline/forumline/forum/sqlcdb"
)

// GetProfile returns a profile by ID.
func (s *Store) GetProfile(ctx context.Context, id uuid.UUID) (*oapi.Profile, error) {
	row, err := s.Q.GetProfile(ctx, id)
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
func (s *Store) GetProfilesByIDs(ctx context.Context, ids []uuid.UUID) ([]oapi.Profile, error) {
	rows, err := s.Q.GetProfilesByIDs(ctx, ids)
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
	id, err := s.Q.GetProfileIDByForumlineID(ctx, &forumlineID)
	if err != nil {
		return nil, err
	}
	return s.GetProfile(ctx, id)
}

// GetProfileIDByForumlineID returns just the profile UUID for a forumline_id.
func (s *Store) GetProfileIDByForumlineID(ctx context.Context, forumlineID string) (uuid.UUID, error) {
	return s.Q.GetProfileIDByForumlineID(ctx, &forumlineID)
}

// GetProfileIDByForumlineIDUnlinked returns the profile UUID if forumline_id is null/empty.
func (s *Store) GetProfileIDByForumlineIDUnlinked(ctx context.Context, userID uuid.UUID) (uuid.UUID, error) {
	return s.Q.GetProfileIDByForumlineIDUnlinked(ctx, userID)
}

// UsernameExists checks if a username is taken.
func (s *Store) UsernameExists(ctx context.Context, username string) (bool, error) {
	return s.Q.UsernameExists(ctx, username)
}

// UpsertProfileFull creates or updates a profile with username.
func (s *Store) UpsertProfileFull(ctx context.Context, id uuid.UUID, username string, displayName, avatarURL *string) error {
	now := time.Now()
	return s.Q.UpsertProfileFull(ctx, sqlcdb.UpsertProfileFullParams{
		ID:          id,
		Username:    username,
		DisplayName: displayName,
		AvatarUrl:   avatarURL,
		CreatedAt:   now,
	})
}

// UpdateProfilePartial performs a dynamic partial update on a profile.
func (s *Store) UpdateProfilePartial(ctx context.Context, userID uuid.UUID, setClauses []string, args []interface{}) error {
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
func (s *Store) ClearForumlineID(ctx context.Context, userID uuid.UUID) error {
	return s.Q.ClearForumlineID(ctx, userID)
}

// SetForumlineID sets the forumline_id on a profile.
func (s *Store) SetForumlineID(ctx context.Context, userID uuid.UUID, forumlineID string) error {
	return s.Q.SetForumlineID(ctx, sqlcdb.SetForumlineIDParams{
		ForumlineID: &forumlineID,
		ID:          userID,
	})
}

// EnsureProfileWithForumlineID creates or updates a profile with the forumline_id set.
func (s *Store) EnsureProfileWithForumlineID(ctx context.Context, userID uuid.UUID, identity *ForumlineIdentity) error {
	return s.Q.EnsureProfileWithForumlineID(ctx, sqlcdb.EnsureProfileWithForumlineIDParams{
		ID:          userID,
		Username:    identity.Username,
		DisplayName: &identity.DisplayName,
		ForumlineID: &identity.ForumlineID,
	})
}

// GetForumlineID returns the forumline_id for a user.
func (s *Store) GetForumlineID(ctx context.Context, userID uuid.UUID) (*string, error) {
	t, err := s.Q.GetForumlineID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return t, nil
}

// IsAdmin returns whether a user is an admin.
func (s *Store) IsAdmin(ctx context.Context, userID uuid.UUID) (bool, error) {
	return s.Q.IsAdmin(ctx, userID)
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

// GetUserIDByUsername returns a user UUID for a username (case-insensitive).
func (s *Store) GetUserIDByUsername(ctx context.Context, username string) (uuid.UUID, error) {
	return s.Q.GetUserIDByUsername(ctx, username)
}

// GetUsername returns the username for a user ID.
func (s *Store) GetUsername(ctx context.Context, userID uuid.UUID) (string, error) {
	return s.Q.GetUsername(ctx, userID)
}

// UpdateDisplayName updates a profile's display_name.
func (s *Store) UpdateDisplayName(ctx context.Context, userID uuid.UUID, displayName string) error {
	return s.Q.UpdateDisplayName(ctx, sqlcdb.UpdateDisplayNameParams{
		DisplayName: &displayName,
		ID:          userID,
	})
}

// UpdateDisplayNameAndAvatar updates display_name and avatar_url.
func (s *Store) UpdateDisplayNameAndAvatar(ctx context.Context, userID uuid.UUID, displayName, avatarURL string) error {
	return s.Q.UpdateDisplayNameAndAvatar(ctx, sqlcdb.UpdateDisplayNameAndAvatarParams{
		DisplayName: &displayName,
		AvatarUrl:   &avatarURL,
		ID:          userID,
	})
}

// CreateProfileSimple creates a basic profile with username.
func (s *Store) CreateProfileSimple(ctx context.Context, id uuid.UUID, username, displayName string) error {
	return s.Q.CreateProfileSimple(ctx, sqlcdb.CreateProfileSimpleParams{
		ID:          id,
		Username:    username,
		DisplayName: &displayName,
	})
}

// CreateProfileHosted creates a profile for hosted mode with a generated UUID
// and stores the forumline_id for identity linking.
func (s *Store) CreateProfileHosted(ctx context.Context, identity *ForumlineIdentity) (uuid.UUID, error) {
	id := uuid.New()
	err := s.Q.CreateProfileHosted(ctx, sqlcdb.CreateProfileHostedParams{
		ID:          id,
		Username:    identity.Username,
		DisplayName: &identity.DisplayName,
		AvatarUrl:   &identity.AvatarURL,
		ForumlineID: &identity.ForumlineID,
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
