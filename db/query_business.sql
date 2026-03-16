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
