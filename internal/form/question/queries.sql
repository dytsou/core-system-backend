-- name: Create :one
INSERT INTO questions (section_id, required, type, title, description, metadata, "order", source_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
    RETURNING *;

-- name: Update :one
UPDATE questions
SET required = $3, type = $4, title = $5, description = $6, metadata = $7, source_id = $8, updated_at = now()
WHERE section_id = $1 AND id = $2
    RETURNING *;

-- name: UpdateOrder :one
WITH old_order AS (
    SELECT "order" as old_pos FROM questions WHERE id = $2 AND section_id = $1
),
shifted AS (
    UPDATE questions
    SET "order" = CASE
        -- Moving down: shift questions between old and new position up
        WHEN $3 > (SELECT old_pos FROM old_order) 
             AND "order" > (SELECT old_pos FROM old_order) 
             AND "order" <= $3
        THEN "order" - 1
        -- Moving up: shift questions between new and old position down
        WHEN $3 < (SELECT old_pos FROM old_order)
             AND "order" >= $3
             AND "order" < (SELECT old_pos FROM old_order)
        THEN "order" + 1
        ELSE "order"
    END,
    updated_at = now()
    WHERE section_id = $1 AND id != $2
    RETURNING id
)
UPDATE questions q
SET "order" = $3, updated_at = now()
WHERE q.id = $2 AND q.section_id = $1
RETURNING *;

-- name: DeleteAndReorder :exec
WITH deleted_row AS (
    DELETE FROM questions q
    WHERE q.section_id = $1 AND q.id = $2
    RETURNING "order" as old_order, section_id
)
UPDATE questions q
SET "order" = q."order" - 1
FROM deleted_row
WHERE q.section_id = deleted_row.section_id
  AND "order" > deleted_row.old_order;

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