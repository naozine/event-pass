-- +goose Up
ALTER TABLE events ADD COLUMN group_name TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE events DROP COLUMN group_name;
