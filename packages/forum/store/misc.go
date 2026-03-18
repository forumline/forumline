package store

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/forumline/forumline/forum/oapi"
	"github.com/forumline/forumline/forum/sqlcdb"
)

// ListChannelFollows returns category UUIDs the user follows.
func (s *Store) ListChannelFollows(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
	return s.Q.ListChannelFollows(ctx, userID)
}

// AddChannelFollow adds a channel follow.
func (s *Store) AddChannelFollow(ctx context.Context, userID, categoryID uuid.UUID) error {
	return s.Q.AddChannelFollow(ctx, sqlcdb.AddChannelFollowParams{
		UserID:     userID,
		CategoryID: categoryID,
	})
}

// RemoveChannelFollow removes a channel follow.
func (s *Store) RemoveChannelFollow(ctx context.Context, userID, categoryID uuid.UUID) error {
	return s.Q.RemoveChannelFollow(ctx, sqlcdb.RemoveChannelFollowParams{
		UserID:     userID,
		CategoryID: categoryID,
	})
}

// ListNotificationPrefs returns a user's notification preferences.
func (s *Store) ListNotificationPrefs(ctx context.Context, userID uuid.UUID) ([]oapi.NotificationPreference, error) {
	rows, err := s.Q.ListNotificationPrefs(ctx, userID)
	if err != nil {
		return nil, err
	}
	prefs := make([]oapi.NotificationPreference, 0, len(rows))
	for _, r := range rows {
		prefs = append(prefs, oapi.NotificationPreference{
			Category: oapi.NotificationPreferenceCategory(r.Category),
			Enabled:  r.Enabled,
		})
	}
	return prefs, nil
}

// UpsertNotificationPref creates or updates a notification preference.
func (s *Store) UpsertNotificationPref(ctx context.Context, userID uuid.UUID, category string, enabled bool) error {
	return s.Q.UpsertNotificationPref(ctx, sqlcdb.UpsertNotificationPrefParams{
		UserID:    userID,
		Category:  category,
		Enabled:   enabled,
		UpdatedAt: time.Now().UTC(),
	})
}

// GetAdminStats returns admin dashboard statistics.
func (s *Store) GetAdminStats(ctx context.Context) (oapi.AdminStats, error) {
	var stats oapi.AdminStats
	var err error

	users, err := s.Q.CountProfiles(ctx)
	if err != nil {
		return stats, err
	}
	stats.TotalUsers = int(users)

	threads, err := s.Q.CountThreads(ctx)
	if err != nil {
		return stats, err
	}
	stats.TotalThreads = int(threads)

	posts, err := s.Q.CountPosts(ctx)
	if err != nil {
		return stats, err
	}
	stats.TotalPosts = int(posts)

	return stats, nil
}
