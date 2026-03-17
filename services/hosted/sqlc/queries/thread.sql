-- name: ListThreads :many
SELECT t.id, t.category_id, t.author_id, t.title, t.slug, t.content, t.image_url,
       t.is_pinned, t.is_locked, t.view_count, t.post_count,
       COALESCE(t.last_post_at, t.created_at) AS last_post_at, t.created_at, t.updated_at,
       p.id AS author_id_2, p.username AS author_username, p.display_name AS author_display_name,
       p.avatar_url AS author_avatar_url, p.bio AS author_bio, p.website AS author_website,
       p.is_admin AS author_is_admin, p.forumline_id AS author_forumline_id,
       p.created_at AS author_created_at, p.updated_at AS author_updated_at,
       c.id AS cat_id, c.name AS cat_name, c.slug AS cat_slug,
       c.description AS cat_description, c.sort_order AS cat_sort_order, c.created_at AS cat_created_at
FROM threads t
JOIN profiles p ON p.id = t.author_id
JOIN categories c ON c.id = t.category_id
ORDER BY t.is_pinned DESC, t.last_post_at DESC NULLS LAST LIMIT $1;

-- name: GetThread :one
SELECT t.id, t.category_id, t.author_id, t.title, t.slug, t.content, t.image_url,
       t.is_pinned, t.is_locked, t.view_count, t.post_count,
       COALESCE(t.last_post_at, t.created_at) AS last_post_at, t.created_at, t.updated_at,
       p.id AS author_id_2, p.username AS author_username, p.display_name AS author_display_name,
       p.avatar_url AS author_avatar_url, p.bio AS author_bio, p.website AS author_website,
       p.is_admin AS author_is_admin, p.forumline_id AS author_forumline_id,
       p.created_at AS author_created_at, p.updated_at AS author_updated_at,
       c.id AS cat_id, c.name AS cat_name, c.slug AS cat_slug,
       c.description AS cat_description, c.sort_order AS cat_sort_order, c.created_at AS cat_created_at
FROM threads t
JOIN profiles p ON p.id = t.author_id
JOIN categories c ON c.id = t.category_id
WHERE t.id = $1;

-- name: ListThreadsByCategory :many
SELECT t.id, t.category_id, t.author_id, t.title, t.slug, t.content, t.image_url,
       t.is_pinned, t.is_locked, t.view_count, t.post_count,
       COALESCE(t.last_post_at, t.created_at) AS last_post_at, t.created_at, t.updated_at,
       p.id AS author_id_2, p.username AS author_username, p.display_name AS author_display_name,
       p.avatar_url AS author_avatar_url, p.bio AS author_bio, p.website AS author_website,
       p.is_admin AS author_is_admin, p.forumline_id AS author_forumline_id,
       p.created_at AS author_created_at, p.updated_at AS author_updated_at,
       c.id AS cat_id, c.name AS cat_name, c.slug AS cat_slug,
       c.description AS cat_description, c.sort_order AS cat_sort_order, c.created_at AS cat_created_at
FROM threads t
JOIN profiles p ON p.id = t.author_id
JOIN categories c ON c.id = t.category_id
WHERE c.slug = $1 ORDER BY t.is_pinned DESC, t.last_post_at DESC NULLS LAST;

-- name: ListUserThreads :many
SELECT t.id, t.category_id, t.author_id, t.title, t.slug, t.content, t.image_url,
       t.is_pinned, t.is_locked, t.view_count, t.post_count,
       COALESCE(t.last_post_at, t.created_at) AS last_post_at, t.created_at, t.updated_at,
       p.id AS author_id_2, p.username AS author_username, p.display_name AS author_display_name,
       p.avatar_url AS author_avatar_url, p.bio AS author_bio, p.website AS author_website,
       p.is_admin AS author_is_admin, p.forumline_id AS author_forumline_id,
       p.created_at AS author_created_at, p.updated_at AS author_updated_at,
       c.id AS cat_id, c.name AS cat_name, c.slug AS cat_slug,
       c.description AS cat_description, c.sort_order AS cat_sort_order, c.created_at AS cat_created_at
FROM threads t
JOIN profiles p ON p.id = t.author_id
JOIN categories c ON c.id = t.category_id
WHERE t.author_id = $1 ORDER BY t.created_at DESC LIMIT 10;

-- name: SearchThreads :many
SELECT t.id, t.category_id, t.author_id, t.title, t.slug, t.content, t.image_url,
       t.is_pinned, t.is_locked, t.view_count, t.post_count,
       COALESCE(t.last_post_at, t.created_at) AS last_post_at, t.created_at, t.updated_at,
       p.id AS author_id_2, p.username AS author_username, p.display_name AS author_display_name,
       p.avatar_url AS author_avatar_url, p.bio AS author_bio, p.website AS author_website,
       p.is_admin AS author_is_admin, p.forumline_id AS author_forumline_id,
       p.created_at AS author_created_at, p.updated_at AS author_updated_at,
       c.id AS cat_id, c.name AS cat_name, c.slug AS cat_slug,
       c.description AS cat_description, c.sort_order AS cat_sort_order, c.created_at AS cat_created_at
FROM threads t
JOIN profiles p ON p.id = t.author_id
JOIN categories c ON c.id = t.category_id
WHERE t.title ILIKE $1 ORDER BY t.created_at DESC LIMIT 20;

-- name: CreateThread :one
INSERT INTO threads (category_id, author_id, title, slug, content, image_url, post_count, last_post_at, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, 1, $7, $7, $7)
RETURNING id;

-- name: GetThreadOwnership :one
SELECT t.author_id, COALESCE(p.is_admin, false) AS is_admin
FROM threads t LEFT JOIN profiles p ON p.id = @user_id
WHERE t.id = @thread_id;

-- name: UpdateThreadStats :exec
UPDATE threads SET last_post_at = $2, post_count = post_count + 1, updated_at = $2 WHERE id = $1;
