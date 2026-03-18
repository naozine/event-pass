-- NOTE: Do not use Japanese in sqlc source files (causes code generation bugs)

-- name: ListProjects :many
SELECT * FROM projects ORDER BY created_at DESC;

-- name: CreateProject :one
INSERT INTO projects (name)
VALUES (?)
RETURNING *;

-- name: GetProject :one
SELECT * FROM projects WHERE id = ? LIMIT 1;

-- name: UpdateProject :one
UPDATE projects
SET name = ?
WHERE id = ?
RETURNING *;

-- name: DeleteProject :exec
DELETE FROM projects
WHERE id = ?;

-- Events

-- name: ListPublishedEvents :many
SELECT * FROM events
WHERE is_published = 1 AND event_date >= datetime('now')
ORDER BY event_date ASC;

-- name: ListAllEvents :many
SELECT * FROM events ORDER BY event_date DESC;

-- name: GetEvent :one
SELECT * FROM events WHERE id = ? LIMIT 1;

-- name: CreateEvent :one
INSERT INTO events (title, description, venue, event_date, capacity, is_published, custom_fields)
VALUES (?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: UpdateEvent :one
UPDATE events
SET title = ?, description = ?, venue = ?, event_date = ?, capacity = ?, is_published = ?, custom_fields = ?, updated_at = CURRENT_TIMESTAMP
WHERE id = ?
RETURNING *;

-- name: DeleteEvent :exec
DELETE FROM events WHERE id = ?;

-- name: CountRegistrationsByEvent :one
SELECT COUNT(*) FROM registrations
WHERE event_id = ? AND status = 'registered';

-- Registrations

-- name: CreateRegistration :one
INSERT INTO registrations (event_id, user_id, name, status, custom_fields)
VALUES (?, ?, ?, 'registered', ?)
RETURNING *;

-- name: GetRegistrationByID :one
SELECT * FROM registrations WHERE id = ? LIMIT 1;

-- name: ListRegistrationsByEvent :many
SELECT r.*, u.email as user_email
FROM registrations r
JOIN users u ON r.user_id = u.id
WHERE r.event_id = ?
ORDER BY r.created_at DESC;

-- name: ListRegistrationsByUser :many
SELECT r.*, e.title as event_title, e.event_date, e.venue
FROM registrations r
JOIN events e ON r.event_id = e.id
WHERE r.user_id = ?
ORDER BY e.event_date DESC;

-- name: UpdateRegistrationStatus :one
UPDATE registrations
SET status = ?
WHERE id = ?
RETURNING *;

-- name: GetRegistrationByEventAndUser :one
SELECT * FROM registrations
WHERE event_id = ? AND user_id = ?
LIMIT 1;
