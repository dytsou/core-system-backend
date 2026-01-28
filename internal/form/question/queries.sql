-- name: Create :one
INSERT INTO questions (section_id, required, type, title, description, metadata, "order", source_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
    RETURNING *;

-- name: Update :one
UPDATE questions
SET required = $3, type = $4, title = $5, description = $6, metadata = $7, "order" = $8, source_id = $9, updated_at = now()
WHERE section_id = $1 AND id = $2
    RETURNING *;

-- name: Delete :exec
DELETE FROM questions WHERE section_id = $1 AND id = $2;

-- name: ListByFormID :many
SELECT
    s.id as section_id,
    s.form_id,
    s.title,
    s.progress,
    s.description,
    s.created_at,
    s.updated_at,
    q.id,
    q.required,
    q.type,
    q.title as question_title,
    q.description as question_description,
    q.metadata,
    q."order",
    q.source_id,
    q.created_at as question_created_at,
    q.updated_at as question_updated_at
FROM sections s
LEFT JOIN questions q ON s.id = q.section_id
WHERE s.form_id = $1
ORDER BY
    s.id ASC,
    q."order" ASC;

-- name: GetByID :one
SELECT * FROM questions WHERE id = $1;