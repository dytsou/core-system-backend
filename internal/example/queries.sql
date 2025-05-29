-- name: GetAll :many
SELECT * FROM scoreboards;

-- name: GetByID :one
SELECT * FROM scoreboards WHERE id = $1;

-- name: Create :one
INSERT INTO scoreboards (name) VALUES ($1) RETURNING *;

-- name: Update :one
UPDATE scoreboards SET name = $2, updated_at = CURRENT_TIMESTAMP WHERE id = $1 RETURNING *;

-- name: Delete :exec
DELETE FROM scoreboards WHERE id = $1;