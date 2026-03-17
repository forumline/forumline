-- name: GetProfile :one
SELECT id, username, display_name, avatar_url, bio, website, is_admin, forumline_id, created_at, updated_at
FROM profiles WHERE id = $1;

-- name: GetProfileByUsername :one
SELECT id, username, display_name, avatar_url, bio, website, is_admin, forumline_id, created_at, updated_at
FROM profiles WHERE username = $1;

-- name: GetProfilesByIDs :many
SELECT id, username, display_name, avatar_url, bio, website, is_admin, forumline_id, created_at, updated_at
FROM profiles WHERE id = ANY(@ids::uuid[]);

-- name: GetProfileIDByForumlineID :one
SELECT id FROM profiles WHERE forumline_id = $1;

-- name: GetProfileIDByForumlineIDUnlinked :one
SELECT id FROM profiles WHERE id = $1 AND (forumline_id IS NULL OR forumline_id = '');

-- name: UsernameExists :one
SELECT EXISTS(SELECT 1 FROM profiles WHERE username = $1);

-- name: UpsertProfileFull :exec
INSERT INTO profiles (id, username, display_name, avatar_url, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $5)
ON CONFLICT (id) DO UPDATE SET
  username = EXCLUDED.username,
  display_name = EXCLUDED.display_name,
  avatar_url = EXCLUDED.avatar_url,
  updated_at = EXCLUDED.updated_at;

-- name: ClearForumlineID :exec
UPDATE profiles SET forumline_id = NULL, updated_at = NOW() WHERE id = $1;

-- name: SetForumlineID :exec
UPDATE profiles SET forumline_id = $1 WHERE id = $2;

-- name: EnsureProfileWithForumlineID :exec
INSERT INTO profiles (id, username, display_name, forumline_id)
VALUES ($1, $2, $3, $4)
ON CONFLICT (id) DO UPDATE SET forumline_id = $4, display_name = $3;

-- name: GetForumlineID :one
SELECT forumline_id FROM profiles WHERE id = $1;

-- name: IsAdmin :one
SELECT is_admin FROM profiles WHERE id = $1;

-- name: ListProfiles :many
SELECT id, username, display_name, avatar_url, bio, website, is_admin, forumline_id, created_at, updated_at
FROM profiles ORDER BY created_at DESC LIMIT $1;

-- name: GetUserIDByUsername :one
SELECT id FROM profiles WHERE lower(username) = $1;

-- name: GetUsername :one
SELECT username FROM profiles WHERE id = $1;

-- name: UpdateDisplayName :exec
UPDATE profiles SET display_name = $1 WHERE id = $2;

-- name: UpdateDisplayNameAndAvatar :exec
UPDATE profiles SET display_name = $1, avatar_url = $2, updated_at = now() WHERE id = $3;

-- name: CreateProfileSimple :exec
INSERT INTO profiles (id, username, display_name)
VALUES ($1, $2, $3)
ON CONFLICT (id) DO UPDATE SET username = $2, display_name = $3;

-- name: CreateProfileHosted :exec
INSERT INTO profiles (id, username, display_name, avatar_url, forumline_id)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (id) DO UPDATE SET forumline_id = $5, display_name = $3;
