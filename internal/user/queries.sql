-- name: Create :one
INSERT INTO users (name, username, avatar_url, role, email)
VALUES ($1, $2, $3, $4, $5) 
RETURNING *;

-- name: ExistsByID :one
SELECT EXISTS(SELECT 1 FROM users WHERE id = $1);

-- name: GetByID :one
SELECT * FROM users WHERE id = $1;

-- name: Update :one
UPDATE users
SET name = $2, username = $3, avatar_url = $4, 
    email = CASE 
        WHEN $5 IS NOT NULL AND array_length($5, 1) > 0 THEN
            (SELECT array_agg(DISTINCT unnest_email)
             FROM unnest(array_cat(email, $5)) AS unnest_email
             WHERE unnest_email != '')
        ELSE email
    END,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: CreateAuth :one
INSERT INTO auth (user_id, provider, provider_id)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetUserIDByAuth :one
SELECT user_id FROM auth WHERE provider = $1 AND provider_id = $2;

-- name: UserExistsByAuth :one
SELECT EXISTS(SELECT 1 FROM auth WHERE provider = $1 AND provider_id = $2);
