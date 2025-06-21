-- name: CreateUser :one
INSERT INTO users (name, username, avatar_url, role)
VALUES ($1, $2, $3, $4) 
RETURNING *;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;



-- name: UpdateUser :one
UPDATE users SET 
  name = $2,
  username = $3,
  avatar_url = $4,
  role = $5,
  updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING *;

-- name: DeleteUser :exec
DELETE FROM users WHERE id = $1;

-- name: CreateAuth :one
INSERT INTO auth (user_id, provider, provider_id)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetAuthByUserIDAndProvider :one
SELECT * FROM auth WHERE user_id = $1 AND provider = $2;

-- name: GetAuthByProviderAndProviderID :one
SELECT * FROM auth WHERE provider = $1 AND provider_id = $2;

-- name: GetAuthByUserID :many
SELECT * FROM auth WHERE user_id = $1;

-- name: DeleteAuthByUserID :exec
DELETE FROM auth WHERE user_id = $1;

-- name: DeleteAuthByUserIDAndProvider :exec
DELETE FROM auth WHERE user_id = $1 AND provider = $2;

-- name: GetUserByAuth :one
SELECT u.* 
FROM users u
INNER JOIN auth a ON u.id = a.user_id
WHERE a.provider = $1 AND a.provider_id = $2
LIMIT 1;