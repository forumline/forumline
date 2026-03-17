package store

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/forumline/forumline/forum/model"
	"github.com/forumline/forumline/forum/sqlcdb"
)

// ListBookmarks returns a user's bookmarks with full thread data.
func (s *Store) ListBookmarks(ctx context.Context, userID string) ([]model.Bookmark, error) {
	rows, err := s.Q.ListBookmarks(ctx, pgUUID(userID))
	if err != nil {
		return nil, err
	}
	bookmarks := make([]model.Bookmark, 0, len(rows))
	for _, r := range rows {
		bookmarks = append(bookmarks, bookmarkRowToModel(r))
	}
	return bookmarks, nil
}

// GetBookmarkStatus returns the bookmark ID if it exists.
func (s *Store) GetBookmarkStatus(ctx context.Context, userID, threadID string) (*string, error) {
	id, err := s.Q.GetBookmarkStatus(ctx, sqlcdb.GetBookmarkStatusParams{
		UserID:   pgUUID(userID),
		ThreadID: pgUUID(threadID),
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	s2 := uuidStr(id)
	return &s2, nil
}

// AddBookmark adds a bookmark.
func (s *Store) AddBookmark(ctx context.Context, userID, threadID string) error {
	return s.Q.AddBookmark(ctx, sqlcdb.AddBookmarkParams{
		UserID:   pgUUID(userID),
		ThreadID: pgUUID(threadID),
	})
}

// RemoveBookmark removes a bookmark by thread ID.
func (s *Store) RemoveBookmark(ctx context.Context, userID, threadID string) error {
	return s.Q.RemoveBookmark(ctx, sqlcdb.RemoveBookmarkParams{
		UserID:   pgUUID(userID),
		ThreadID: pgUUID(threadID),
	})
}

// RemoveBookmarkByID removes a bookmark by bookmark ID.
func (s *Store) RemoveBookmarkByID(ctx context.Context, userID, bookmarkID string) error {
	return s.Q.RemoveBookmarkByID(ctx, sqlcdb.RemoveBookmarkByIDParams{
		ID:     pgUUID(bookmarkID),
		UserID: pgUUID(userID),
	})
}
