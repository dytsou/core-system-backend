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

-- name: CreateNode :one
WITH latest_workflow AS (
    SELECT wv.id, wv.is_active, wv.form_id, wv.workflow
    FROM workflow_versions AS wv
    WHERE wv.form_id = $1
    ORDER BY wv.updated_at DESC
    LIMIT 1
    FOR UPDATE
),
new_section AS (
    INSERT INTO sections (form_id, title, progress)
    SELECT lw.form_id, 'New Section', 'draft'
    FROM latest_workflow AS lw
    WHERE @type::node_type = 'section'
    RETURNING id
),
node_id AS (
    SELECT COALESCE((SELECT id FROM new_section), gen_random_uuid()) AS id
),
new_node AS (
    SELECT jsonb_build_object(
        'id', node_id.id,
        'label', 'New ' || initcap(@type::text),
        'type', @type::node_type
    ) AS node
    FROM node_id
),
updated AS (
    UPDATE workflow_versions AS wv
    SET workflow = lw.workflow || jsonb_build_array(new_node.node),
        last_editor = $2,
        updated_at = now()
    FROM latest_workflow AS lw, new_node
    WHERE wv.id = lw.id 
      AND lw.is_active = false
    RETURNING wv.workflow, wv.id, wv.form_id, wv.last_editor, wv.is_active, wv.created_at, wv.updated_at
),
created AS (
    INSERT INTO workflow_versions (form_id, last_editor, workflow)
    SELECT $1, $2, lw.workflow || jsonb_build_array(new_node.node)
    FROM latest_workflow AS lw, new_node
    WHERE lw.is_active = true
    RETURNING workflow, id, form_id, last_editor, is_active, created_at, updated_at
)
SELECT (SELECT node->>'id' FROM new_node)::uuid AS node_id FROM updated, new_node
UNION ALL
SELECT (SELECT node->>'id' FROM new_node)::uuid AS node_id FROM created, new_node;