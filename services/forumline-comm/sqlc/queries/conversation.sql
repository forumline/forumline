-- name: ListConversations :many
SELECT
    c.id, c.is_group, c.name,
    COALESCE(c.last_message_content, '') AS last_message,
    COALESCE(c.last_message_at, c.created_at) AS last_message_time,
    COALESCE(cm.last_read_seq, 0)::bigint AS last_read_seq
FROM forumline_conversations c
JOIN forumline_conversation_members cm ON cm.conversation_id = c.id AND cm.user_id = @user_id
ORDER BY COALESCE(c.last_message_at, c.created_at) DESC
LIMIT 100;

-- name: GetConversation :one
SELECT c.id, c.is_group, c.name,
    COALESCE(c.last_message_content, '') AS last_message,
    COALESCE(c.last_message_at, c.created_at) AS last_message_time,
    COALESCE(cm.last_read_seq, 0)::bigint AS last_read_seq
FROM forumline_conversations c
JOIN forumline_conversation_members cm ON cm.conversation_id = c.id AND cm.user_id = @user_id
WHERE c.id = @conversation_id;

-- name: FetchConversationMembers :many
SELECT cm.conversation_id, cm.user_id, p.username, p.display_name, p.avatar_url
FROM forumline_conversation_members cm
JOIN forumline_profiles p ON p.id = cm.user_id
WHERE cm.conversation_id = ANY(@conversation_ids::uuid[]);

-- name: GetConversationMemberIDs :many
SELECT user_id FROM forumline_conversation_members WHERE conversation_id = $1;

-- name: IsConversationMember :one
SELECT EXISTS(SELECT 1 FROM forumline_conversation_members WHERE conversation_id = $1 AND user_id = $2);

-- name: TouchConversationWithMessage :exec
UPDATE forumline_conversations
SET updated_at = now(),
    last_message_content = @content,
    last_message_sender_id = @sender_id,
    last_message_at = @message_at
WHERE id = @conversation_id;

-- name: MarkReadSeq :exec
UPDATE forumline_conversation_members SET last_read_seq = $1
WHERE conversation_id = $2 AND user_id = $3;

-- name: Find1to1Conversation :one
SELECT c.id FROM forumline_conversations c
WHERE c.is_group = false
  AND EXISTS(SELECT 1 FROM forumline_conversation_members cm1 WHERE cm1.conversation_id = c.id AND cm1.user_id = @user_id)
  AND EXISTS(SELECT 1 FROM forumline_conversation_members cm2 WHERE cm2.conversation_id = c.id AND cm2.user_id = @other_user_id)
  AND (SELECT count(*) FROM forumline_conversation_members cm3 WHERE cm3.conversation_id = c.id) = 2;

-- name: CreateConversation :one
INSERT INTO forumline_conversations (is_group, name, created_by) VALUES ($1, $2, $3) RETURNING id;

-- name: InsertConversationMember :exec
INSERT INTO forumline_conversation_members (conversation_id, user_id) VALUES ($1, $2);

-- name: Insert1to1Members :exec
INSERT INTO forumline_conversation_members (conversation_id, user_id) VALUES ($1, $2), ($1, $3);

-- name: IsGroupConversation :one
SELECT c.is_group FROM forumline_conversations c
JOIN forumline_conversation_members cm ON cm.conversation_id = c.id AND cm.user_id = @user_id
WHERE c.id = @conversation_id;

-- name: UpdateConversationName :exec
UPDATE forumline_conversations SET name = $1 WHERE id = $2;

-- name: AddConversationMembers :exec
INSERT INTO forumline_conversation_members (conversation_id, user_id)
SELECT $1, p.id FROM forumline_profiles p WHERE p.id = ANY(@member_ids::text[])
ON CONFLICT DO NOTHING;

-- name: RemoveConversationMembers :exec
DELETE FROM forumline_conversation_members WHERE conversation_id = $1 AND user_id = ANY(@member_ids::text[]);

-- name: LeaveConversation :exec
DELETE FROM forumline_conversation_members WHERE conversation_id = $1 AND user_id = $2;

-- name: GetMemberLastReadSeq :one
SELECT COALESCE(last_read_seq, 0)::bigint AS last_read_seq
FROM forumline_conversation_members
WHERE conversation_id = @conversation_id AND user_id = @user_id;
