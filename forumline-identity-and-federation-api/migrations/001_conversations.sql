-- Migration: Add conversations and conversation_members tables,
-- migrate existing direct messages from the old (sender_id, recipient_id) model
-- to the new conversation-based model.
--
-- Run this BEFORE deploying the new Go binary.

BEGIN;

-- 1. Create new tables
CREATE TABLE IF NOT EXISTS forumline_conversations (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  is_group BOOLEAN NOT NULL DEFAULT false,
  name TEXT,
  created_by UUID REFERENCES forumline_profiles(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS forumline_conversation_members (
  conversation_id UUID NOT NULL REFERENCES forumline_conversations(id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES forumline_profiles(id) ON DELETE CASCADE,
  joined_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  last_read_at TIMESTAMPTZ NOT NULL DEFAULT '1970-01-01',
  PRIMARY KEY (conversation_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_convo_members_user ON forumline_conversation_members(user_id);

-- 2. Add conversation_id column to messages (nullable initially)
ALTER TABLE forumline_direct_messages ADD COLUMN IF NOT EXISTS conversation_id UUID REFERENCES forumline_conversations(id) ON DELETE CASCADE;

-- 3. Create a conversation for each distinct pair of users
INSERT INTO forumline_conversations (id, is_group, created_at)
SELECT gen_random_uuid(), false, min(created_at)
FROM forumline_direct_messages
GROUP BY least(sender_id, recipient_id), greatest(sender_id, recipient_id);

-- 4. Create a temp mapping table to link pairs to conversation IDs
CREATE TEMP TABLE dm_pair_map AS
SELECT
  c.id AS conversation_id,
  sub.user_a,
  sub.user_b,
  sub.min_created
FROM (
  SELECT
    least(sender_id, recipient_id) AS user_a,
    greatest(sender_id, recipient_id) AS user_b,
    min(created_at) AS min_created
  FROM forumline_direct_messages
  GROUP BY least(sender_id, recipient_id), greatest(sender_id, recipient_id)
) sub
JOIN forumline_conversations c ON c.created_at = sub.min_created AND c.is_group = false;

-- 5. Insert conversation members
INSERT INTO forumline_conversation_members (conversation_id, user_id, last_read_at)
SELECT conversation_id, user_a, '1970-01-01'
FROM dm_pair_map
ON CONFLICT DO NOTHING;

INSERT INTO forumline_conversation_members (conversation_id, user_id, last_read_at)
SELECT conversation_id, user_b, '1970-01-01'
FROM dm_pair_map
ON CONFLICT DO NOTHING;

-- 6. Update last_read_at based on existing read flags
-- For each member, set last_read_at to the latest message they received that was marked read
UPDATE forumline_conversation_members cm
SET last_read_at = COALESCE(
  (SELECT max(dm.created_at) FROM forumline_direct_messages dm
   JOIN dm_pair_map dpm ON dpm.conversation_id = cm.conversation_id
   WHERE dm.recipient_id = cm.user_id AND dm.read = true
     AND ((dm.sender_id = dpm.user_a AND dm.recipient_id = dpm.user_b)
       OR (dm.sender_id = dpm.user_b AND dm.recipient_id = dpm.user_a))),
  '1970-01-01'
);

-- 7. Backfill conversation_id on all existing messages
UPDATE forumline_direct_messages dm
SET conversation_id = dpm.conversation_id
FROM dm_pair_map dpm
WHERE least(dm.sender_id, dm.recipient_id) = dpm.user_a
  AND greatest(dm.sender_id, dm.recipient_id) = dpm.user_b;

-- 8. Make conversation_id NOT NULL
ALTER TABLE forumline_direct_messages ALTER COLUMN conversation_id SET NOT NULL;

-- 9. Drop old columns and constraints
ALTER TABLE forumline_direct_messages DROP CONSTRAINT IF EXISTS sender_not_recipient;
ALTER TABLE forumline_direct_messages DROP COLUMN IF EXISTS recipient_id;
ALTER TABLE forumline_direct_messages DROP COLUMN IF EXISTS read;

-- 10. Update indexes
DROP INDEX IF EXISTS idx_forumline_dms_recipient;
DROP INDEX IF EXISTS idx_forumline_dms_unread;
-- Recreate conversation index on new column
DROP INDEX IF EXISTS idx_forumline_dms_conversation;
CREATE INDEX idx_forumline_dms_conversation ON forumline_direct_messages(conversation_id, created_at DESC);

-- 11. Add updated_at trigger for conversations
CREATE TRIGGER forumline_conversations_updated_at
  BEFORE UPDATE ON forumline_conversations
  FOR EACH ROW EXECUTE FUNCTION update_updated_at();

-- 12. Update NOTIFY triggers
CREATE OR REPLACE FUNCTION notify_dm_changes() RETURNS TRIGGER AS $$
DECLARE
  member_ids UUID[];
BEGIN
  SELECT array_agg(user_id) INTO member_ids
  FROM forumline_conversation_members
  WHERE conversation_id = NEW.conversation_id;

  PERFORM pg_notify('dm_changes', json_build_object(
    'conversation_id', NEW.conversation_id,
    'sender_id', NEW.sender_id,
    'member_ids', member_ids,
    'id', NEW.id,
    'content', NEW.content,
    'created_at', NEW.created_at
  )::text);
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION notify_new_dm() RETURNS TRIGGER AS $$
DECLARE
  member_ids UUID[];
BEGIN
  SELECT array_agg(user_id) INTO member_ids
  FROM forumline_conversation_members
  WHERE conversation_id = NEW.conversation_id;

  PERFORM pg_notify('push_dm', json_build_object(
    'conversation_id', NEW.conversation_id,
    'sender_id', NEW.sender_id,
    'member_ids', member_ids,
    'content', NEW.content
  )::text);
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Clean up
DROP TABLE IF EXISTS dm_pair_map;

COMMIT;
