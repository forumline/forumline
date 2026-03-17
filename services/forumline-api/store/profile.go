package store

import (
	"context"
	"fmt"
	"strings"

	"github.com/forumline/forumline/services/forumline-api/model"
	"github.com/forumline/forumline/services/forumline-api/sqlcdb"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (s *Store) GetProfile(ctx context.Context, id string) (*model.Profile, error) {
	row, err := s.Q.GetProfile(ctx, id)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &model.Profile{
		ID:               row.ID,
		Username:         row.Username,
		DisplayName:      row.DisplayName,
		AvatarURL:        pgtextPtr(row.AvatarUrl),
		Bio:              pgtextPtr(row.Bio),
		StatusMessage:    row.StatusMessage,
		OnlineStatus:     row.OnlineStatus,
		ShowOnlineStatus: row.ShowOnlineStatus,
	}, nil
}

func (s *Store) CreateProfile(ctx context.Context, id, username, displayName, avatarURL string) error {
	return s.Q.CreateProfile(ctx, sqlcdb.CreateProfileParams{
		ID:          id,
		Username:    username,
		DisplayName: displayName,
		AvatarUrl:   pgtype.Text{String: avatarURL, Valid: avatarURL != ""},
	})
}

func (s *Store) UsernameExists(ctx context.Context, username string) (bool, error) {
	return s.Q.UsernameExists(ctx, username)
}

// UpdateProfile uses dynamic SQL — can't be expressed in sqlc.
func (s *Store) UpdateProfile(ctx context.Context, id string, sets map[string]interface{}) error {
	if len(sets) == 0 {
		return nil
	}
	clauses := make([]string, 0, len(sets))
	args := make([]interface{}, 0, len(sets)+1)
	i := 1
	for col, val := range sets {
		clauses = append(clauses, fmt.Sprintf("%s = $%d", col, i))
		args = append(args, val)
		i++
	}
	args = append(args, id)
	query := fmt.Sprintf("UPDATE forumline_profiles SET %s WHERE id = $%d", strings.Join(clauses, ", "), i)
	_, err := s.Pool.Exec(ctx, query, args...)
	return err
}

func (s *Store) DeleteUser(ctx context.Context, id string) error {
	return s.Q.DeleteUser(ctx, id)
}

func (s *Store) SearchProfiles(ctx context.Context, query, excludeUserID string) ([]model.ProfileSearchResult, error) {
	pattern := "%" + query + "%"
	rows, err := s.Q.SearchProfiles(ctx, sqlcdb.SearchProfilesParams{
		ID:       excludeUserID,
		Username: pattern,
	})
	if err != nil {
		return nil, err
	}
	results := make([]model.ProfileSearchResult, len(rows))
	for i, r := range rows {
		results[i] = model.ProfileSearchResult{
			ID:          r.ID,
			Username:    r.Username,
			DisplayName: pgtextPtr(pgtype.Text{String: r.DisplayName, Valid: r.DisplayName != ""}),
			AvatarURL:   pgtextPtr(r.AvatarUrl),
		}
	}
	return results, nil
}

func (s *Store) ProfileExists(ctx context.Context, id string) (bool, error) {
	return s.Q.ProfileExists(ctx, id)
}

func (s *Store) FetchProfilesByIDs(ctx context.Context, ids []string) (map[string]*model.Profile, error) {
	profiles := make(map[string]*model.Profile)
	if len(ids) == 0 {
		return profiles, nil
	}
	rows, err := s.Q.FetchProfilesByIDs(ctx, ids)
	if err != nil {
		return profiles, err
	}
	for _, r := range rows {
		profiles[r.ID] = &model.Profile{
			ID:          r.ID,
			Username:    r.Username,
			DisplayName: r.DisplayName,
			AvatarURL:   pgtextPtr(r.AvatarUrl),
		}
	}
	return profiles, nil
}

