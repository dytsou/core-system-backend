-- name: CreateUser :one
INSERT INTO users (name, username, avatar_url, role)
VALUES ($1, $2, $3, $4) 
RETURNING *;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: CreateAuth :one
INSERT INTO auth (user_id, provider, provider_id)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetUserByAuth :one
SELECT u.* 
FROM users u
INNER JOIN auth a ON u.id = a.user_id
WHERE a.provider = $1 AND a.provider_id = $2
LIMIT 1;
