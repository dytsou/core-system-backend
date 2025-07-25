-- name: Create :one
INSERT INTO tenants (id, db_strategy)
VALUES ($1, $2)
RETURNING *;

-- name: Get :one
SELECT * FROM tenants WHERE id = $1;

-- name: Update :one
UPDATE tenants
SET db_strategy = $2
WHERE id = $1
RETURNING *;

-- name: Delete :exec
DELETE FROM tenants
WHERE id = $1;