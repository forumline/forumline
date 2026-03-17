package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/forumline/forumline/forum/oapi"
	"github.com/forumline/forumline/forum/sqlcdb"
)

// ListPostsByThread returns posts for a thread ordered by creation time.
func (s *Store) ListPostsByThread(ctx context.Context, threadID string) ([]oapi.Post, error) {
	rows, err := s.Q.ListPostsByThread(ctx, pgUUID(threadID))
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
func (s *Store) ListUserPosts(ctx context.Context, userID string) ([]oapi.Post, error) {
	rows, err := s.Q.ListUserPosts(ctx, pgUUID(userID))
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
func (s *Store) CreatePost(ctx context.Context, threadID, authorID, content string, replyToID *string) (string, error) {
	now := time.Now()
	var replyUUID pgtype.UUID
	if replyToID != nil {
		replyUUID = pgUUID(*replyToID)
	}
	id, err := s.Q.CreatePost(ctx, sqlcdb.CreatePostParams{
		ThreadID:  pgUUID(threadID),
		AuthorID:  pgUUID(authorID),
		Content:   content,
		ReplyToID: replyUUID,
		CreatedAt: pgTimestamp(now),
	})
	if err != nil {
		return "", err
	}
	return uuidStr(id), nil
}

// GetPostAuthor returns the author_id of a post.
func (s *Store) GetPostAuthor(ctx context.Context, postID string) (string, error) {
	id, err := s.Q.GetPostAuthor(ctx, pgUUID(postID))
	if err != nil {
		return "", err
	}
	return uuidStr(id), nil
}
