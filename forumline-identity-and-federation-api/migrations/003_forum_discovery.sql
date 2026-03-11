-- Migration: Add forum discovery features
-- Adds tags, member_count, and search indexes to forumline_forums.
--
-- Run this BEFORE deploying the new Go binary.

BEGIN;

-- 1. Add tags column (text array for categorization)
ALTER TABLE forumline_forums ADD COLUMN IF NOT EXISTS tags TEXT[] DEFAULT '{}';

-- 2. Add member_count column (denormalized for fast sorting/display)
ALTER TABLE forumline_forums ADD COLUMN IF NOT EXISTS member_count INTEGER DEFAULT 0 NOT NULL;
ALTER TABLE forumline_forums ADD CONSTRAINT IF NOT EXISTS member_count_non_negative CHECK (member_count >= 0);

-- 3. Backfill member_count from existing memberships
UPDATE forumline_forums f
SET member_count = sub.cnt
FROM (
  SELECT forum_id, COUNT(*) AS cnt
  FROM forumline_memberships
  GROUP BY forum_id
) sub
WHERE f.id = sub.forum_id;

-- 4. Create trigger to keep member_count in sync
CREATE OR REPLACE FUNCTION update_forum_member_count() RETURNS TRIGGER AS $$
BEGIN
  IF TG_OP = 'INSERT' THEN
    UPDATE forumline_forums SET member_count = member_count + 1 WHERE id = NEW.forum_id;
    RETURN NEW;
  ELSIF TG_OP = 'DELETE' THEN
    UPDATE forumline_forums SET member_count = GREATEST(member_count - 1, 0) WHERE id = OLD.forum_id;
    RETURN OLD;
  END IF;
  RETURN NULL;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS update_forum_member_count_trigger ON forumline_memberships;
CREATE TRIGGER update_forum_member_count_trigger
  AFTER INSERT OR DELETE ON forumline_memberships
  FOR EACH ROW EXECUTE FUNCTION update_forum_member_count();

-- 5. Indexes for discovery queries
CREATE INDEX IF NOT EXISTS idx_forumline_forums_approved ON forumline_forums(approved) WHERE approved = true;
CREATE INDEX IF NOT EXISTS idx_forumline_forums_tags ON forumline_forums USING GIN(tags) WHERE approved = true;
CREATE INDEX IF NOT EXISTS idx_forumline_forums_member_count ON forumline_forums(member_count DESC) WHERE approved = true;

-- 6. Index for recommended forums query (membership lookups by forum_id)
CREATE INDEX IF NOT EXISTS idx_forumline_memberships_forum_id ON forumline_memberships(forum_id);

COMMIT;
