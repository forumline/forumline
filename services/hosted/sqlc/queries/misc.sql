-- name: ListChannelFollows :many
SELECT category_id FROM channel_follows WHERE user_id = $1;

-- name: AddChannelFollow :exec
INSERT INTO channel_follows (user_id, category_id)
VALUES ($1, $2)
ON CONFLICT (user_id, category_id) DO NOTHING;

-- name: RemoveChannelFollow :exec
DELETE FROM channel_follows WHERE user_id = $1 AND category_id = $2;

-- name: ListNotificationPrefs :many
SELECT category, enabled FROM notification_preferences WHERE user_id = $1;

-- name: UpsertNotificationPref :exec
INSERT INTO notification_preferences (user_id, category, enabled, updated_at)
VALUES ($1, $2, $3, $4)
ON CONFLICT (user_id, category)
DO UPDATE SET enabled = $3, updated_at = $4;

-- name: CountProfiles :one
SELECT COUNT(*)::int FROM profiles;

-- name: CountThreads :one
SELECT COUNT(*)::int FROM threads;

-- name: CountPosts :one
SELECT COUNT(*)::int FROM posts;
