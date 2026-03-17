package store

import (
	"context"
	"fmt"
	"time"

	"github.com/forumline/forumline/services/hosted/forum/model"
	"github.com/forumline/forumline/services/hosted/sqlcdb"
)

// ListThreads returns threads ordered by pinned + last_post_at.
func (s *Store) ListThreads(ctx context.Context, limit int) ([]model.Thread, error) {
	rows, err := s.Q.ListThreads(ctx, int32(min(limit, 1000))) //nolint:gosec // bounded
	if err != nil {
		return nil, err
	}
	threads := make([]model.Thread, 0, len(rows))
	for _, r := range rows {
		threads = append(threads, listThreadsRowToModel(r))
	}
	return threads, nil
}

// GetThread returns a single thread by ID.
func (s *Store) GetThread(ctx context.Context, id string) (*model.Thread, error) {
	row, err := s.Q.GetThread(ctx, pgUUID(id))
	if err != nil {
		return nil, err
	}
	t := threadRowToModel(row)
	return &t, nil
}

// ListThreadsByCategory returns threads for a category slug.
func (s *Store) ListThreadsByCategory(ctx context.Context, slug string) ([]model.Thread, error) {
	rows, err := s.Q.ListThreadsByCategory(ctx, slug)
	if err != nil {
		return nil, err
	}
	threads := make([]model.Thread, 0, len(rows))
	for _, r := range rows {
		threads = append(threads, listThreadsByCategoryRowToModel(r))
	}
	return threads, nil
}

// ListUserThreads returns threads authored by a user.
func (s *Store) ListUserThreads(ctx context.Context, userID string) ([]model.Thread, error) {
	rows, err := s.Q.ListUserThreads(ctx, pgUUID(userID))
	if err != nil {
		return nil, err
	}
	threads := make([]model.Thread, 0, len(rows))
	for _, r := range rows {
		threads = append(threads, listUserThreadsRowToModel(r))
	}
	return threads, nil
}

// SearchThreads searches threads by title.
func (s *Store) SearchThreads(ctx context.Context, pattern string) ([]model.Thread, error) {
	rows, err := s.Q.SearchThreads(ctx, pattern)
	if err != nil {
		return nil, err
	}
	threads := make([]model.Thread, 0, len(rows))
	for _, r := range rows {
		threads = append(threads, searchThreadsRowToModel(r))
	}
	return threads, nil
}

// CreateThread inserts a new thread and returns its ID.
func (s *Store) CreateThread(ctx context.Context, categoryID, authorID, title, slug string, content, imageURL *string) (string, error) {
	now := time.Now()
	id, err := s.Q.CreateThread(ctx, sqlcdb.CreateThreadParams{
		CategoryID: pgUUID(categoryID),
		AuthorID:   pgUUID(authorID),
		Title:      title,
		Slug:       slug,
		Content:    optTextToPgtext(content),
		ImageUrl:   optTextToPgtext(imageURL),
		LastPostAt: pgTimestamp(now),
	})
	if err != nil {
		return "", err
	}
	return uuidStr(id), nil
}

// GetThreadOwnership returns the thread author ID and whether the given user is an admin.
func (s *Store) GetThreadOwnership(ctx context.Context, threadID, userID string) (authorID string, isAdmin bool, err error) {
	row, err := s.Q.GetThreadOwnership(ctx, sqlcdb.GetThreadOwnershipParams{
		UserID:   pgUUID(userID),
		ThreadID: pgUUID(threadID),
	})
	if err != nil {
		return "", false, err
	}
	return uuidStr(row.AuthorID), row.IsAdmin, nil
}

// UpdateThread performs a dynamic update on a thread.
func (s *Store) UpdateThread(ctx context.Context, threadID string, setClauses []string, args []interface{}) error {
	query := "UPDATE threads SET "
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

// UpdateThreadStats updates a thread's last_post_at and increments post_count.
func (s *Store) UpdateThreadStats(ctx context.Context, threadID string, now time.Time) error {
	return s.Q.UpdateThreadStats(ctx, sqlcdb.UpdateThreadStatsParams{
		ID:         pgUUID(threadID),
		LastPostAt: pgTimestamp(now),
	})
}

// SetClause returns a SQL SET clause like "col = $N".
func SetClause(col string, argIdx int) string {
	return col + " = $" + fmt.Sprint(argIdx)
}
