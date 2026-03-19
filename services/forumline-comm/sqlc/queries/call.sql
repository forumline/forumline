-- name: GetCalleeFor1to1 :one
SELECT cm2.user_id FROM forumline_conversations c
JOIN forumline_conversation_members cm1 ON cm1.conversation_id = c.id AND cm1.user_id = @user_id
JOIN forumline_conversation_members cm2 ON cm2.conversation_id = c.id AND cm2.user_id != @user_id
WHERE c.id = @conversation_id AND c.is_group = false
AND (SELECT count(*) FROM forumline_conversation_members WHERE conversation_id = c.id) = 2;

-- name: CreateCallRecord :one
INSERT INTO forumline_calls (conversation_id, caller_id, callee_id, status, room_name)
VALUES ($1, $2, $3, 'ringing', $4)
RETURNING id, conversation_id, caller_id, callee_id, status, room_name, created_at;

-- name: GetCallByID :one
SELECT id, conversation_id, caller_id, callee_id, status, room_name, created_at, started_at, ended_at, duration_seconds
FROM forumline_calls WHERE id = $1;

-- name: GetCallByRoomName :one
SELECT id, conversation_id, caller_id, callee_id, status, room_name, created_at, started_at, ended_at, duration_seconds
FROM forumline_calls WHERE room_name = $1 AND status IN ('ringing', 'active');

-- name: UpdateCallStatus :exec
UPDATE forumline_calls SET status = $1 WHERE id = $2;

-- name: ActivateCall :exec
UPDATE forumline_calls SET status = 'active', started_at = now() WHERE id = $1;

-- name: EndCallWithDuration :exec
UPDATE forumline_calls SET status = $1, ended_at = now(), duration_seconds = $2 WHERE id = $3;

-- name: EndCallWithoutDuration :exec
UPDATE forumline_calls SET status = $1, ended_at = now() WHERE id = $2;

-- name: IsCallParticipant :one
SELECT EXISTS(SELECT 1 FROM forumline_calls
WHERE id = $1 AND (caller_id = $2 OR callee_id = $2));
