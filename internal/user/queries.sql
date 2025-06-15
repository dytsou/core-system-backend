-- name: CreateUser :one
INSERT INTO users (name, username, avatar_url, role) 
VALUES ($1, $2, $3, $4) 
RETURNING *;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: GetUserByUsername :one
SELECT * FROM users WHERE username = $1;

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
INSERT INTO users (id, name, username, avatar_url, role) 
VALUES ($1, $2, $3, $4, $5) 
ON CONFLICT (id) DO UPDATE SET 
  name = $2, 
  username = $3, 
  avatar_url = $4, 
  role = $5
RETURNING *;