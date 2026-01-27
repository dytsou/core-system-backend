-- name: Create :one
INSERT INTO questions (section_id, required, type, title, description, metadata, "order", source_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
    RETURNING *;

-- name: Update :one
UPDATE questions
SET required = $3, type = $4, title = $5, description = $6, metadata = $7, "order" = $8, source_id = $8, updated_at = now()
WHERE section_id = $1 AND id = $2
    RETURNING *;

-- name: Delete :exec
DELETE FROM questions WHERE section_id = $1 AND id = $2;

-- name: ListByFormID :many
SELECT
    s.form_id,
    s.title,
    s.progress,
    s.description,
    s.created_at,
    s.updated_at,
    q.*
FROM sections s
JOIN questions q ON s.id = q.section_id
WHERE s.form_id = $1
ORDER BY
    s.id ASC,
    q."order" ASC;

-- name: GetByID :one
SELECT * FROM questions WHERE id = $1;