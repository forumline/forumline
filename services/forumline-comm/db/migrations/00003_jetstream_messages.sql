-- +goose Up
-- Move DM message persistence to NATS JetStream.
-- Conversation metadata and membership stay in Postgres.

-- Add denormalized last-message columns for efficient ListConversations.
ALTER TABLE forumline_conversations ADD COLUMN IF NOT EXISTS last_message_content TEXT;
ALTER TABLE forumline_conversations ADD COLUMN IF NOT EXISTS last_message_sender_id TEXT;
ALTER TABLE forumline_conversations ADD COLUMN IF NOT EXISTS last_message_at TIMESTAMPTZ;

-- Replace timestamp-based read tracking with JetStream sequence tracking.
ALTER TABLE forumline_conversation_members ADD COLUMN IF NOT EXISTS last_read_seq BIGINT DEFAULT 0;

-- Messages now live in JetStream — drop the Postgres table.
DROP TABLE IF EXISTS forumline_direct_messages;

-- +goose Down
-- Re-create the messages table (message data cannot be recovered from JetStream).
CREATE TABLE IF NOT EXISTS forumline_direct_messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id UUID NOT NULL REFERENCES forumline_conversations(id) ON DELETE CASCADE,
    sender_id TEXT NOT NULL REFERENCES forumline_profiles(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_dm_conversation_id ON forumline_direct_messages(conversation_id, created_at);
ALTER TABLE forumline_conversation_members DROP COLUMN IF EXISTS last_read_seq;
ALTER TABLE forumline_conversations DROP COLUMN IF EXISTS last_message_at;
ALTER TABLE forumline_conversations DROP COLUMN IF EXISTS last_message_sender_id;
ALTER TABLE forumline_conversations DROP COLUMN IF EXISTS last_message_content;
