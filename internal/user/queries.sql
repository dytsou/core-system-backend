-- name: Create :one
INSERT INTO users (name, username, avatar_url, role)
VALUES ($1, $2, $3, $4) 
RETURNING *;

-- name: ExistsByID :one
SELECT EXISTS(SELECT 1 FROM users WHERE id = $1);

-- name: GetByID :one
SELECT * FROM users WHERE id = $1;

-- name: Update :one
UPDATE users
SET name = $2, username = $3, avatar_url = $4, 
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: CreateAuth :one
INSERT INTO auth (user_id, provider, provider_id)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetIDByAuth :one
SELECT user_id FROM auth WHERE provider = $1 AND provider_id = $2;

-- name: ExistsByAuth :one
SELECT EXISTS(SELECT 1 FROM auth WHERE provider = $1 AND provider_id = $2);

-- name: CreateEmail :one
INSERT INTO emails (user_id, value, provider, provider_id)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetEmailsByID :many
SELECT * FROM emails WHERE user_id = $1;

-- name: GetEmailByAddress :one
SELECT * FROM emails WHERE value = $1;
