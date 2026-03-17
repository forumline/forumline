package store

import (
	"context"
	"time"

	"github.com/forumline/forumline/services/forumline-api/model"
	"github.com/forumline/forumline/services/forumline-api/sqlcdb"
	"github.com/jackc/pgx/v5"
)

func (s *Store) ListMemberships(ctx context.Context, userID string) ([]model.Membership, error) {
	rows, err := s.Q.ListMemberships(ctx, userID)
	if err != nil {
		return nil, err
	}

	memberships := make([]model.Membership, 0, len(rows))
	for _, r := range rows {
		m := model.Membership{
			ForumDomain:        r.Domain,
			ForumName:          r.Name,
			ForumIconURL:       pgtextPtr(r.IconUrl),
			APIBase:            r.ApiBase,
			WebBase:            r.WebBase,
			Capabilities:       r.Capabilities,
			MemberCount:        int(r.MemberCount),
			JoinedAt:           r.JoinedAt.Time.Format(time.RFC3339),
			NotificationsMuted: r.NotificationsMuted,
		}
		if r.ForumAuthedAt.Valid {
			s := r.ForumAuthedAt.Time.Format(time.RFC3339)
			m.ForumAuthedAt = &s
		}
		memberships = append(memberships, m)
	}
	if len(memberships) == 0 {
		memberships = []model.Membership{}
	}
	return memberships, nil
}

func (s *Store) UpsertMembership(ctx context.Context, userID, forumID string) error {
	return s.Q.UpsertMembership(ctx, sqlcdb.UpsertMembershipParams{
		UserID:  userID,
		ForumID: pgUUID(forumID),
	})
}

func (s *Store) DeleteMembership(ctx context.Context, userID, forumID string) error {
	return s.Q.DeleteMembership(ctx, sqlcdb.DeleteMembershipParams{
		UserID:  userID,
		ForumID: pgUUID(forumID),
	})
}

func (s *Store) UpdateMembershipAuth(ctx context.Context, userID, forumID string, authed bool) error {
	if authed {
		return s.Q.SetMembershipAuthed(ctx, sqlcdb.SetMembershipAuthedParams{
			UserID:  userID,
			ForumID: pgUUID(forumID),
		})
	}
	return s.Q.ClearMembershipAuthed(ctx, sqlcdb.ClearMembershipAuthedParams{
		UserID:  userID,
		ForumID: pgUUID(forumID),
	})
}

func (s *Store) UpdateMembershipMute(ctx context.Context, userID, forumID string, muted bool) error {
	return s.Q.UpdateMembershipMute(ctx, sqlcdb.UpdateMembershipMuteParams{
		NotificationsMuted: muted,
		UserID:             userID,
		ForumID:            pgUUID(forumID),
	})
}

func (s *Store) GetMembershipJoinDetails(ctx context.Context, forumID, userID string) (map[string]interface{}, error) {
	r, err := s.Q.GetMembershipJoinDetails(ctx, sqlcdb.GetMembershipJoinDetailsParams{
		ID:     pgUUID(forumID),
		UserID: userID,
	})
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"domain": r.Domain, "name": r.Name, "icon_url": pgtextPtr(r.IconUrl),
		"api_base": r.ApiBase, "web_base": r.WebBase, "capabilities": r.Capabilities,
		"joined_at": r.JoinedAt.Time.Format(time.RFC3339), "member_count": int(r.MemberCount),
	}, nil
}

func (s *Store) IsNotificationsMuted(ctx context.Context, userID, forumID string) (bool, error) {
	muted, err := s.Q.IsNotificationsMuted(ctx, sqlcdb.IsNotificationsMutedParams{
		UserID:  userID,
		ForumID: pgUUID(forumID),
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	return muted, nil
}
