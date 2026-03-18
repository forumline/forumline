package store

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/forumline/forumline/forum/oapi"
	"github.com/forumline/forumline/forum/sqlcdb"
)

// ListPostsByThread returns posts for a thread ordered by creation time.
func (s *Store) ListPostsByThread(ctx context.Context, threadID uuid.UUID) ([]oapi.Post, error) {
	rows, err := s.Q.ListPostsByThread(ctx, threadID)
	if err != nil {
		return nil, err
	}
	posts := make([]oapi.Post, 0, len(rows))
	for _, r := range rows {
		posts = append(posts, postRowToOapi(r))
	}
	return posts, nil
}

// ListUserPosts returns posts authored by a user.
func (s *Store) ListUserPosts(ctx context.Context, userID uuid.UUID) ([]oapi.Post, error) {
	rows, err := s.Q.ListUserPosts(ctx, userID)
	if err != nil {
		return nil, err
	}
	posts := make([]oapi.Post, 0, len(rows))
	for _, r := range rows {
		posts = append(posts, listUserPostsRowToOapi(r))
	}
	return posts, nil
}

// SearchPosts searches posts by content.
func (s *Store) SearchPosts(ctx context.Context, pattern string) ([]oapi.Post, error) {
	rows, err := s.Q.SearchPosts(ctx, pattern)
	if err != nil {
		return nil, err
	}
	posts := make([]oapi.Post, 0, len(rows))
	for _, r := range rows {
		posts = append(posts, searchPostsRowToOapi(r))
	}
	return posts, nil
}

// CreatePost inserts a new post and returns its ID.
func (s *Store) CreatePost(ctx context.Context, threadID, authorID uuid.UUID, content string, replyToID *uuid.UUID) (uuid.UUID, error) {
	now := time.Now()
	id, err := s.Q.CreatePost(ctx, sqlcdb.CreatePostParams{
		ThreadID:  threadID,
		AuthorID:  authorID,
		Content:   content,
		ReplyToID: replyToID,
		CreatedAt: pgTimestamp(now),
	})
	if err != nil {
		return uuid.UUID{}, err
	}
	return id, nil
}

// GetPostAuthor returns the author_id of a post.
func (s *Store) GetPostAuthor(ctx context.Context, postID uuid.UUID) (uuid.UUID, error) {
	return s.Q.GetPostAuthor(ctx, postID)
}
