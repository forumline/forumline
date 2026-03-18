package store

import (
	"context"

	"github.com/forumline/forumline/services/forumline-api/sqlcdb"
)

// PushSubscription is an internal type for push subscription data (not in the OpenAPI spec).
type PushSubscription struct {
	Endpoint string
	P256dh   string
	Auth     string
}

func (s *Store) UpsertPushSubscription(ctx context.Context, userID, endpoint, p256dh, auth string) error {
	return s.Q.UpsertPushSubscription(ctx, sqlcdb.UpsertPushSubscriptionParams{
		UserID:   userID,
		Endpoint: endpoint,
		P256dh:   p256dh,
		Auth:     auth,
	})
}

func (s *Store) DeletePushSubscription(ctx context.Context, userID, endpoint string) error {
	return s.Q.DeletePushSubscription(ctx, sqlcdb.DeletePushSubscriptionParams{
		UserID:   userID,
		Endpoint: endpoint,
	})
}

func (s *Store) ListPushSubscriptions(ctx context.Context, userID string) ([]PushSubscription, error) {
	rows, err := s.Q.ListPushSubscriptions(ctx, userID)
	if err != nil {
		return nil, err
	}
	subs := make([]PushSubscription, len(rows))
	for i, r := range rows {
		subs[i] = PushSubscription{
			Endpoint: r.Endpoint,
			P256dh:   r.P256dh,
			Auth:     r.Auth,
		}
	}
	return subs, nil
}

func (s *Store) DeleteStaleEndpoints(ctx context.Context, userID string, endpoints []string) {
	if len(endpoints) == 0 {
		return
	}
	_ = s.Q.DeleteStaleEndpoints(ctx, sqlcdb.DeleteStaleEndpointsParams{
		UserID:    userID,
		Endpoints: endpoints,
	})
}

func (s *Store) GetSenderUsername(ctx context.Context, senderID string) string {
	username, err := s.Q.GetSenderUsername(ctx, senderID)
	if err != nil || username == "" {
		return "someone"
	}
	return username
}

func (s *Store) GetOnlineStatusPreferences(ctx context.Context, userIDs []string) (map[string]bool, error) {
	result := make(map[string]bool)
	rows, err := s.Q.GetOnlineStatusPreferences(ctx, userIDs)
	if err != nil {
		return result, err
	}
	for _, r := range rows {
		if !r.ShowOnlineStatus || r.OnlineStatus == "offline" || r.OnlineStatus == "away" {
			result[r.ID] = false
		}
	}
	return result, nil
}

func (s *Store) UserExists(ctx context.Context, userID string) (bool, error) {
	return s.Q.UserExists(ctx, userID)
}

func (s *Store) CountExistingUsers(ctx context.Context, userIDs []string) (int, error) {
	count, err := s.Q.CountExistingUsers(ctx, userIDs)
	return int(count), err
}
