package store

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/forumline/forumline/services/forumline-api/oapi"
	"github.com/forumline/forumline/services/forumline-api/sqlcdb"
)

func (s *Store) ListMemberships(ctx context.Context, userID string) ([]oapi.Membership, error) {
	rows, err := s.Q.ListMemberships(ctx, userID)
	if err != nil {
		return nil, err
	}

	memberships := make([]oapi.Membership, 0, len(rows))
	for _, r := range rows {
		m := oapi.Membership{
			ForumDomain:        r.Domain,
			ForumName:          r.Name,
			ForumIconUrl:       r.IconUrl,
			ApiBase:            r.ApiBase,
			WebBase:            r.WebBase,
			Capabilities:       r.Capabilities,
			MemberCount:        int(r.MemberCount),
			JoinedAt:           r.JoinedAt.Format(time.RFC3339),
			NotificationsMuted: r.NotificationsMuted,
		}
		if r.ForumAuthedAt != nil {
			ts := r.ForumAuthedAt.Format(time.RFC3339)
			m.ForumAuthedAt = &ts
		}
		memberships = append(memberships, m)
	}
	if len(memberships) == 0 {
		memberships = []oapi.Membership{}
	}
	return memberships, nil
}

func (s *Store) UpsertMembership(ctx context.Context, userID string, forumID uuid.UUID) error {
	return s.Q.UpsertMembership(ctx, sqlcdb.UpsertMembershipParams{
		UserID:  userID,
		ForumID: forumID,
	})
}

func (s *Store) DeleteMembership(ctx context.Context, userID string, forumID uuid.UUID) error {
	return s.Q.DeleteMembership(ctx, sqlcdb.DeleteMembershipParams{
		UserID:  userID,
		ForumID: forumID,
	})
}

func (s *Store) UpdateMembershipAuth(ctx context.Context, userID string, forumID uuid.UUID, authed bool) error {
	if authed {
		return s.Q.SetMembershipAuthed(ctx, sqlcdb.SetMembershipAuthedParams{
			UserID:  userID,
			ForumID: forumID,
		})
	}
	return s.Q.ClearMembershipAuthed(ctx, sqlcdb.ClearMembershipAuthedParams{
		UserID:  userID,
		ForumID: forumID,
	})
}

func (s *Store) UpdateMembershipMute(ctx context.Context, userID string, forumID uuid.UUID, muted bool) error {
	return s.Q.UpdateMembershipMute(ctx, sqlcdb.UpdateMembershipMuteParams{
		NotificationsMuted: muted,
		UserID:             userID,
		ForumID:            forumID,
	})
}

func (s *Store) GetMembershipJoinDetails(ctx context.Context, forumID uuid.UUID, userID string) (map[string]interface{}, error) {
	r, err := s.Q.GetMembershipJoinDetails(ctx, sqlcdb.GetMembershipJoinDetailsParams{
		ID:     forumID,
		UserID: userID,
	})
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"domain": r.Domain, "name": r.Name, "icon_url": r.IconUrl,
		"api_base": r.ApiBase, "web_base": r.WebBase, "capabilities": r.Capabilities,
		"joined_at": r.JoinedAt.Format(time.RFC3339), "member_count": int(r.MemberCount),
	}, nil
}

func (s *Store) IsNotificationsMuted(ctx context.Context, userID string, forumID uuid.UUID) (bool, error) {
	muted, err := s.Q.IsNotificationsMuted(ctx, sqlcdb.IsNotificationsMutedParams{
		UserID:  userID,
		ForumID: forumID,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	return muted, nil
}
