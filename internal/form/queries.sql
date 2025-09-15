-- name: Create :one
INSERT INTO forms (title, description, unit_id, last_editor)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: Update :one
UPDATE forms
SET title = $2, description = $3, last_editor = $4, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: Delete :exec
DELETE FROM forms WHERE id = $1;

-- name: GetByID :one
SELECT * FROM forms WHERE id = $1;

-- name: List :many
SELECT * FROM forms ORDER BY updated_at DESC;

-- name: ListByUnit :many
SELECT * FROM forms
WHERE unit_id = $1
ORDER BY updated_at DESC;

-- name: SetStatus :one
UPDATE forms
SET status = $2, last_editor = $3, updated_at = now()
WHERE id = $1
RETURNING *;