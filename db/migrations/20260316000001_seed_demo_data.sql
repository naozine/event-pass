-- +goose Up

-- Demo admin user
INSERT OR IGNORE INTO users (email, name, role, is_active)
VALUES ('admin@example.com', 'Admin User', 'admin', 1);

-- Demo attendee user
INSERT OR IGNORE INTO users (email, name, role, is_active)
VALUES ('attendee@example.com', 'Jane Doe', 'viewer', 1);

-- +goose Down
DELETE FROM users WHERE email IN ('admin@example.com', 'attendee@example.com');
