-- name: Get :one
SELECT workflow, id, form_id, last_editor, is_active, created_at, updated_at
FROM workflow_versions
WHERE form_id = $1
ORDER BY updated_at DESC
LIMIT 1;
