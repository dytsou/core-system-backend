-- name: GetByID :one
SELECT * FROM refresh_tokens WHERE id = $1;

-- name: GetUserIDByRefreshToken :one
SELECT user_id FROM refresh_tokens WHERE id = $1 AND is_active = TRUE;

-- name: Create :one
INSERT INTO refresh_tokens (user_id, expiration_date) VALUES ($1, $2) RETURNING *;

-- name: Inactivate :one
UPDATE refresh_tokens SET is_active = FALSE WHERE id = $1 RETURNING *;

-- name: InactivateByUserID :execrows
UPDATE refresh_tokens SET is_active = FALSE WHERE user_id = $1 RETURNING *;

-- name: DeleteExpired :execrows
DELETE FROM refresh_tokens WHERE expiration_date < NOW() OR is_active = FALSE;