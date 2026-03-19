-- +goose Up
ALTER TABLE forumline_calls ADD COLUMN IF NOT EXISTS room_name TEXT;

-- +goose Down
ALTER TABLE forumline_calls DROP COLUMN IF EXISTS room_name;
