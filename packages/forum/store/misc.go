package store

import (
	"context"
	"time"

	"github.com/forumline/forumline/forum/model"
	"github.com/forumline/forumline/forum/sqlcdb"
)

// ListChannelFollows returns category IDs the user follows.
func (s *Store) ListChannelFollows(ctx context.Context, userID string) ([]string, error) {
	rows, err := s.Q.ListChannelFollows(ctx, pgUUID(userID))
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, uuidStr(r))
	}
	return ids, nil
}

// AddChannelFollow adds a channel follow.
func (s *Store) AddChannelFollow(ctx context.Context, userID, categoryID string) error {
	return s.Q.AddChannelFollow(ctx, sqlcdb.AddChannelFollowParams{
		UserID:     pgUUID(userID),
		CategoryID: pgUUID(categoryID),
	})
}

// RemoveChannelFollow removes a channel follow.
func (s *Store) RemoveChannelFollow(ctx context.Context, userID, categoryID string) error {
	return s.Q.RemoveChannelFollow(ctx, sqlcdb.RemoveChannelFollowParams{
		UserID:     pgUUID(userID),
		CategoryID: pgUUID(categoryID),
	})
}

// ListNotificationPrefs returns a user's notification preferences.
func (s *Store) ListNotificationPrefs(ctx context.Context, userID string) ([]model.NotificationPreference, error) {
	rows, err := s.Q.ListNotificationPrefs(ctx, pgUUID(userID))
	if err != nil {
		return nil, err
	}
	prefs := make([]model.NotificationPreference, 0, len(rows))
	for _, r := range rows {
		prefs = append(prefs, model.NotificationPreference{
			Category: r.Category,
			Enabled:  r.Enabled,
		})
	}
	return prefs, nil
}

// UpsertNotificationPref creates or updates a notification preference.
func (s *Store) UpsertNotificationPref(ctx context.Context, userID, category string, enabled bool) error {
	return s.Q.UpsertNotificationPref(ctx, sqlcdb.UpsertNotificationPrefParams{
		UserID:    pgUUID(userID),
		Category:  category,
		Enabled:   enabled,
		UpdatedAt: pgTimestamp(time.Now().UTC()),
	})
}

// GetAdminStats returns admin dashboard statistics.
func (s *Store) GetAdminStats(ctx context.Context) (model.AdminStats, error) {
	var stats model.AdminStats
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
