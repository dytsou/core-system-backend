-- name: CreateOAuthProvider :one
INSERT INTO oauth_providers (user_id, provider, client_id, client_secret, redirect_uri) 
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetOAuthProviderByUserID :many
SELECT * FROM oauth_providers WHERE user_id = $1;

-- name: DeleteOAuthProvider :exec
DELETE FROM oauth_providers WHERE id = $1;