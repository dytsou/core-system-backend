-- name: Create :one
INSERT INTO tenants (id, db_strategy)
VALUES ($1, $2)
RETURNING *;

-- name: Get :one
SELECT * FROM tenants WHERE id = $1;
