-- +goose Up
ALTER TABLE events ADD COLUMN custom_fields TEXT NOT NULL DEFAULT '[]';
ALTER TABLE registrations ADD COLUMN custom_fields TEXT NOT NULL DEFAULT '[]';

-- +goose Down
ALTER TABLE events DROP COLUMN custom_fields;
ALTER TABLE registrations DROP COLUMN custom_fields;
