-- name: CreateMessage :one
INSERT INTO inbox_message (posted_by, type, content_id)
VALUES ($1, $2, $3)
RETURNING *;

-- name: CreateUserInboxBulk :many
INSERT INTO user_inbox_messages (user_id, message_id)
SELECT unnest($1::uuid[]), $2::uuid
RETURNING *;

-- name: GetByID :one
SELECT *
FROM user_inbox_messages uim
JOIN inbox_message im ON uim.message_id = im.id
WHERE uim.id = $1 AND uim.user_id = $2;

-- name: List :many
SELECT *
FROM user_inbox_messages uim
JOIN inbox_message im ON uim.message_id = im.id
WHERE uim.user_id = $1;

-- name: UpdateByID :one
UPDATE user_inbox_messages AS uim
SET is_read = $3, is_starred = $4, is_archived = $5
FROM inbox_message AS im
WHERE uim.message_id = im.id AND uim.id = $1 AND uim.user_id = $2
RETURNING *;
