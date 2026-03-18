package store

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/forumline/forumline/forum/oapi"
	"github.com/forumline/forumline/forum/sqlcdb"
)

// ListBookmarks returns a user's bookmarks with full thread data.
func (s *Store) ListBookmarks(ctx context.Context, userID uuid.UUID) ([]oapi.Bookmark, error) {
	rows, err := s.Q.ListBookmarks(ctx, userID)
	if err != nil {
		return nil, err
	}
	bookmarks := make([]oapi.Bookmark, 0, len(rows))
	for _, r := range rows {
		bookmarks = append(bookmarks, bookmarkRowToOapi(r))
	}
	return bookmarks, nil
}

// GetBookmarkStatus returns the bookmark UUID if it exists, or nil if not found.
func (s *Store) GetBookmarkStatus(ctx context.Context, userID, threadID uuid.UUID) (*uuid.UUID, error) {
	id, err := s.Q.GetBookmarkStatus(ctx, sqlcdb.GetBookmarkStatusParams{
		UserID:   userID,
		ThreadID: threadID,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &id, nil
}

// AddBookmark adds a bookmark.
func (s *Store) AddBookmark(ctx context.Context, userID, threadID uuid.UUID) error {
	return s.Q.AddBookmark(ctx, sqlcdb.AddBookmarkParams{
		UserID:   userID,
		ThreadID: threadID,
	})
}

// RemoveBookmark removes a bookmark by thread ID.
func (s *Store) RemoveBookmark(ctx context.Context, userID, threadID uuid.UUID) error {
	return s.Q.RemoveBookmark(ctx, sqlcdb.RemoveBookmarkParams{
		UserID:   userID,
		ThreadID: threadID,
	})
}

// RemoveBookmarkByID removes a bookmark by bookmark ID.
func (s *Store) RemoveBookmarkByID(ctx context.Context, userID, bookmarkID uuid.UUID) error {
	return s.Q.RemoveBookmarkByID(ctx, sqlcdb.RemoveBookmarkByIDParams{
		ID:     bookmarkID,
		UserID: userID,
	})
}
