-- name: GetById :one
SELECT *
FROM user_inbox_messages uim
JOIN inbox_message im ON uim.message_id = im.id
WHERE uim.id = $1 AND uim.user_id = $2;

-- name: List :many
SELECT *
FROM user_inbox_messages uim
JOIN inbox_message im ON uim.message_id = im.id
WHERE uim.user_id = $1;

-- name: UpdateById :one
UPDATE user_inbox_messages AS uim
SET is_read = $3, is_starred = $4, is_archived = $5
FROM inbox_message AS im
WHERE uim.message_id = im.id AND uim.id = $1 AND uim.user_id = $2
RETURNING *;