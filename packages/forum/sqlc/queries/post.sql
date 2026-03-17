-- name: ListPostsByThread :many
SELECT po.id, po.thread_id, po.author_id, po.content, po.reply_to_id, po.created_at, po.updated_at,
       p.id AS author_id_2, p.username AS author_username, p.display_name AS author_display_name,
       p.avatar_url AS author_avatar_url, p.bio AS author_bio, p.website AS author_website,
       p.is_admin AS author_is_admin, p.forumline_id AS author_forumline_id,
       p.created_at AS author_created_at, p.updated_at AS author_updated_at
FROM posts po
JOIN profiles p ON p.id = po.author_id
WHERE po.thread_id = $1 ORDER BY po.created_at ASC;

-- name: ListUserPosts :many
SELECT po.id, po.thread_id, po.author_id, po.content, po.reply_to_id, po.created_at, po.updated_at,
       p.id AS author_id_2, p.username AS author_username, p.display_name AS author_display_name,
       p.avatar_url AS author_avatar_url, p.bio AS author_bio, p.website AS author_website,
       p.is_admin AS author_is_admin, p.forumline_id AS author_forumline_id,
       p.created_at AS author_created_at, p.updated_at AS author_updated_at
FROM posts po
JOIN profiles p ON p.id = po.author_id
WHERE po.author_id = $1 ORDER BY po.created_at DESC LIMIT 20;

-- name: SearchPosts :many
SELECT po.id, po.thread_id, po.author_id, po.content, po.reply_to_id, po.created_at, po.updated_at,
       p.id AS author_id_2, p.username AS author_username, p.display_name AS author_display_name,
       p.avatar_url AS author_avatar_url, p.bio AS author_bio, p.website AS author_website,
       p.is_admin AS author_is_admin, p.forumline_id AS author_forumline_id,
       p.created_at AS author_created_at, p.updated_at AS author_updated_at
FROM posts po
JOIN profiles p ON p.id = po.author_id
WHERE po.content ILIKE $1 ORDER BY po.created_at DESC LIMIT 20;

-- name: CreatePost :one
INSERT INTO posts (thread_id, author_id, content, reply_to_id, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $5)
RETURNING id;

-- name: GetPostAuthor :one
SELECT author_id FROM posts WHERE id = $1;
