-- name: Get :one
SELECT workflow, id, form_id, last_editor, is_active, created_at, updated_at
FROM workflow_versions
WHERE form_id = $1
ORDER BY updated_at DESC
LIMIT 1;

-- name: Update :one
WITH latest AS (
    SELECT wv.id, wv.is_active, wv.form_id
    FROM workflow_versions AS wv
    WHERE wv.form_id = $1
    ORDER BY wv.updated_at DESC
    LIMIT 1
    FOR UPDATE
),
updated AS (
    UPDATE workflow_versions AS wv
    SET workflow = $3, last_editor = $2, updated_at = now()
    FROM latest
    WHERE wv.id = latest.id 
      AND latest.is_active = false
    RETURNING wv.workflow, wv.id, wv.form_id, wv.last_editor, wv.is_active, wv.created_at, wv.updated_at
),
created AS (
    INSERT INTO workflow_versions (form_id, last_editor, workflow)
    SELECT $1, $2, $3
    FROM latest
    WHERE latest.is_active = true
    RETURNING workflow, id, form_id, last_editor, is_active, created_at, updated_at
)
SELECT * FROM updated
UNION ALL
SELECT * FROM created;
