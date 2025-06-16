-- name: CreateUser :one
INSERT INTO users (name, username, avatar_url, role, oauth_provider, oauth_user_id) 
VALUES ($1, $2, $3, $4, $5, $6) 
RETURNING *;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: GetUserByUsername :one
SELECT * FROM users WHERE username = $1 AND oauth_provider = $2;

-- name: GetUserByOAuthID :one
SELECT * FROM users WHERE oauth_provider = $1 AND oauth_user_id = $2;

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

-- name: FindOrCreate :one
INSERT INTO users (name, username, avatar_url, role, oauth_provider, oauth_user_id) 
VALUES ($1, $2, $3, $4, $5, $6) 
ON CONFLICT (oauth_provider, oauth_user_id) DO UPDATE SET 
  name = $1, 
  username = $2,
  avatar_url = $3, 
  role = $4,
  updated_at = CURRENT_TIMESTAMP
RETURNING *;