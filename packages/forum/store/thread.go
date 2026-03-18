package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/forumline/forumline/forum/oapi"
	"github.com/forumline/forumline/forum/sqlcdb"
)

// ListThreads returns threads ordered by pinned + last_post_at.
func (s *Store) ListThreads(ctx context.Context, limit int) ([]oapi.Thread, error) {
	rows, err := s.Q.ListThreads(ctx, int32(min(limit, 1000))) //nolint:gosec // bounded
	if err != nil {
		return nil, err
	}
	threads := make([]oapi.Thread, 0, len(rows))
	for _, r := range rows {
		threads = append(threads, listThreadsRowToOapi(r))
	}
	return threads, nil
}

// GetThread returns a single thread by ID.
func (s *Store) GetThread(ctx context.Context, id uuid.UUID) (*oapi.Thread, error) {
	row, err := s.Q.GetThread(ctx, id)
	if err != nil {
		return nil, err
	}
	t := threadRowToOapi(row)
	return &t, nil
}

// ListThreadsByCategory returns threads for a category slug.
func (s *Store) ListThreadsByCategory(ctx context.Context, slug string) ([]oapi.Thread, error) {
	rows, err := s.Q.ListThreadsByCategory(ctx, slug)
	if err != nil {
		return nil, err
	}
	threads := make([]oapi.Thread, 0, len(rows))
	for _, r := range rows {
		threads = append(threads, listThreadsByCategoryRowToOapi(r))
	}
	return threads, nil
}

// ListUserThreads returns threads authored by a user.
func (s *Store) ListUserThreads(ctx context.Context, userID uuid.UUID) ([]oapi.Thread, error) {
	rows, err := s.Q.ListUserThreads(ctx, userID)
	if err != nil {
		return nil, err
	}
	threads := make([]oapi.Thread, 0, len(rows))
	for _, r := range rows {
		threads = append(threads, listUserThreadsRowToOapi(r))
	}
	return threads, nil
}

// SearchThreads searches threads by title.
func (s *Store) SearchThreads(ctx context.Context, pattern string) ([]oapi.Thread, error) {
	rows, err := s.Q.SearchThreads(ctx, pattern)
	if err != nil {
		return nil, err
	}
	threads := make([]oapi.Thread, 0, len(rows))
	for _, r := range rows {
		threads = append(threads, searchThreadsRowToOapi(r))
	}
	return threads, nil
}

// CreateThread inserts a new thread and returns its ID.
func (s *Store) CreateThread(ctx context.Context, categoryID, authorID uuid.UUID, title, slug string, content, imageURL *string) (uuid.UUID, error) {
	now := time.Now()
	id, err := s.Q.CreateThread(ctx, sqlcdb.CreateThreadParams{
		CategoryID: categoryID,
		AuthorID:   authorID,
		Title:      title,
		Slug:       slug,
		Content:    content,
		ImageUrl:   imageURL,
		LastPostAt: &now,
	})
	if err != nil {
		return uuid.UUID{}, err
	}
	return id, nil
}

// GetThreadOwnership returns the thread author ID and whether the given user is an admin.
func (s *Store) GetThreadOwnership(ctx context.Context, threadID, userID uuid.UUID) (authorID uuid.UUID, isAdmin bool, err error) {
	row, err := s.Q.GetThreadOwnership(ctx, sqlcdb.GetThreadOwnershipParams{
		UserID:   userID,
		ThreadID: threadID,
	})
	if err != nil {
		return uuid.UUID{}, false, err
	}
	return row.AuthorID, row.IsAdmin, nil
}

// UpdateThread performs a dynamic update on a thread.
func (s *Store) UpdateThread(ctx context.Context, threadID uuid.UUID, setClauses []string, args []interface{}) error {
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
func (s *Store) UpdateThreadStats(ctx context.Context, threadID uuid.UUID, now time.Time) error {
	return s.Q.UpdateThreadStats(ctx, sqlcdb.UpdateThreadStatsParams{
		ID:         threadID,
		LastPostAt: &now,
	})
}

// SetClause returns a SQL SET clause like "col = $N".
func SetClause(col string, argIdx int) string {
	return col + " = $" + fmt.Sprint(argIdx)
}
