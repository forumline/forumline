-- name: ListBookmarks :many
SELECT b.id, b.created_at AS bookmark_created_at,
       t.id AS thread_id, t.category_id, t.author_id, t.title, t.slug, t.content, t.image_url,
       t.is_pinned, t.is_locked, t.view_count, t.post_count,
       COALESCE(t.last_post_at, t.created_at) AS last_post_at, t.created_at AS thread_created_at, t.updated_at AS thread_updated_at,
       p.id AS author_id_2, p.username AS author_username, p.display_name AS author_display_name,
       p.avatar_url AS author_avatar_url, p.bio AS author_bio, p.website AS author_website,
       p.is_admin AS author_is_admin, p.forumline_id AS author_forumline_id,
       p.created_at AS author_created_at, p.updated_at AS author_updated_at,
       c.id AS cat_id, c.name AS cat_name, c.slug AS cat_slug,
       c.description AS cat_description, c.sort_order AS cat_sort_order, c.created_at AS cat_created_at
FROM bookmarks b
JOIN threads t ON t.id = b.thread_id
JOIN profiles p ON p.id = t.author_id
JOIN categories c ON c.id = t.category_id
WHERE b.user_id = $1
ORDER BY b.created_at DESC;

-- name: GetBookmarkStatus :one
SELECT id FROM bookmarks WHERE user_id = $1 AND thread_id = $2;

-- name: AddBookmark :exec
INSERT INTO bookmarks (user_id, thread_id) VALUES ($1, $2) ON CONFLICT DO NOTHING;

-- name: RemoveBookmark :exec
DELETE FROM bookmarks WHERE user_id = $1 AND thread_id = $2;

-- name: RemoveBookmarkByID :exec
DELETE FROM bookmarks WHERE id = $1 AND user_id = $2;
