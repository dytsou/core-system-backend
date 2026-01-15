-- name: Get :one
SELECT workflow, id, form_id, last_editor, is_active, created_at, updated_at
FROM workflow_versions
WHERE form_id = $1
ORDER BY updated_at DESC
LIMIT 1;

-- name: Update :one
WITH latest_workflow AS (
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
    FROM latest_workflow AS lw
    WHERE wv.id = lw.id 
      AND lw.is_active = false
    RETURNING wv.workflow, wv.id, wv.form_id, wv.last_editor, wv.is_active, wv.created_at, wv.updated_at
),
created AS (
    INSERT INTO workflow_versions (form_id, last_editor, workflow)
    SELECT $1, $2, $3
    FROM latest_workflow AS lw
    WHERE lw.is_active = true
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
    RETURNING wv.id, wv.form_id, wv.last_editor, wv.is_active, wv.workflow, wv.created_at, wv.updated_at
),
created AS (
    INSERT INTO workflow_versions (form_id, last_editor, workflow)
    SELECT $1, $2, lw.workflow || jsonb_build_array(new_node.node)
    FROM latest_workflow AS lw, new_node
    WHERE lw.is_active = true
    RETURNING id, form_id, last_editor, is_active, workflow, created_at, updated_at
)
SELECT 
    (SELECT node->>'id' FROM new_node)::uuid AS node_id,
    (SELECT node->>'type' FROM new_node)::node_type AS node_type,
    (SELECT node->>'label' FROM new_node) AS node_label,
    u.workflow AS workflow
FROM updated AS u, new_node
UNION ALL
SELECT 
    (SELECT node->>'id' FROM new_node)::uuid AS node_id,
    (SELECT node->>'type' FROM new_node)::node_type AS node_type,
    (SELECT node->>'label' FROM new_node) AS node_label,
    c.workflow AS workflow
FROM created AS c, new_node;

-- name: DeleteNode :one
-- Deletes a node from the workflow and nullifies all references to it in other nodes
WITH latest_workflow AS (
    SELECT wv.id, wv.is_active, wv.form_id, wv.workflow
    FROM workflow_versions AS wv
    WHERE wv.form_id = $1
    ORDER BY wv.updated_at DESC
    LIMIT 1
    FOR UPDATE
),
-- Convert deleted node ID to text format for JSONB comparison
deleted_node_id AS (
    SELECT to_jsonb(@node_id::uuid)::text AS deleted_id
),
-- Extract information about the node to be deleted
node_to_delete AS (
    SELECT 
        node->>'id' AS node_id,
        node->>'type' AS node_type
    FROM latest_workflow AS lw,
    LATERAL jsonb_array_elements(COALESCE(lw.workflow, '[]'::jsonb)) AS node
    WHERE node->>'id' = (SELECT deleted_id FROM deleted_node_id)
    LIMIT 1
),
-- Delete associated section if the node is a section type
deleted_section AS (
    DELETE FROM sections
    WHERE id = (SELECT node_id::uuid FROM node_to_delete)
      AND EXISTS (SELECT 1 FROM node_to_delete WHERE node_type = 'section')
    RETURNING id
),
-- Get all nodes except the one being deleted
remaining_nodes AS (
    SELECT node
    FROM latest_workflow AS lw,
    LATERAL jsonb_array_elements(COALESCE(lw.workflow, '[]'::jsonb)) AS node
    WHERE node->>'id' != (SELECT deleted_id FROM deleted_node_id)
),
-- Expand each node into individual key-value pairs
/* 
Example of node_fields_expanded: 
node                    | field_key  | cleaned_value
------------------------|------------|------------
{"id":"node-a",...}     | "id"       | "node-a"
{"id":"node-a",...}     | "type"     | "start"
{"id":"node-a",...}     | "label"    | "Start"
{"id":"node-a",...}     | "next"     | null  ← NULLIFIED!
{"id":"node-c",...}     | "id"       | "node-c"
{"id":"node-c",...}     | "type"     | "condition"
{"id":"node-c",...}     | "label"    | "Check"
{"id":"node-c",...}     | "nextTrue" | null  ← NULLIFIED!
{"id":"node-c",...}     | "nextFalse"| "node-d"
{"id":"node-d",...}     | "id"       | "node-d"
{"id":"node-d",...}     | "type"     | "end"
{"id":"node-d",...}     | "label"    | "End"
*/
node_fields_expanded AS (
    SELECT 
        node,
        field_key,
        field_value
    FROM remaining_nodes,
    LATERAL jsonb_each(COALESCE(node, '{}'::jsonb)) AS node_fields(field_key, field_value)
),
-- Clean each field: nullify reference fields that point to the deleted node
cleaned_node_fields AS (
    SELECT 
        node,
        field_key,
        CASE 
            -- Nullify reference fields that point to the deleted node
            WHEN field_key IN ('next', 'nextTrue', 'nextFalse') 
             AND field_value::text = (SELECT deleted_id FROM deleted_node_id)
            THEN 'null'::jsonb
            ELSE field_value
        END AS cleaned_value
    FROM node_fields_expanded
),
-- Rebuild nodes from cleaned fields
cleaned_nodes AS (
    SELECT 
        jsonb_object_agg(field_key, cleaned_value) AS cleaned_node
    FROM cleaned_node_fields
    GROUP BY node
),
-- Rebuild the workflow array from cleaned nodes
cleaned_workflow AS (
    SELECT jsonb_agg(cleaned_node) AS workflow
    FROM cleaned_nodes
),
-- Update draft workflow version in place
updated AS (
    UPDATE workflow_versions AS wv
    SET 
        workflow = COALESCE(cw.workflow, '[]'::jsonb),
        last_editor = $2,
        updated_at = now()
    FROM latest_workflow AS lw, cleaned_workflow AS cw
    WHERE wv.id = lw.id 
      AND lw.is_active = false
    RETURNING wv.workflow
),
-- Create new draft version if current version is active
created AS (
    INSERT INTO workflow_versions (form_id, last_editor, workflow)
    SELECT $1, $2, COALESCE(cw.workflow, '[]'::jsonb)
    FROM latest_workflow AS lw, cleaned_workflow AS cw
    WHERE lw.is_active = true
    RETURNING workflow
)
-- Return workflow as JSONB
SELECT workflow FROM updated
UNION ALL
SELECT workflow FROM created;

-- name: Activate :one
WITH deactivated AS (
    UPDATE workflow_versions AS wv
    SET is_active = false
    WHERE wv.form_id = $1
      AND wv.is_active = true
    RETURNING wv.*
),
activated AS (
    UPDATE workflow_versions AS wv
    SET is_active = true, 
        last_editor = $2
    WHERE wv.form_id = $1
      AND wv.is_active = false
      AND wv.updated_at = (SELECT MAX(updated_at) FROM workflow_versions WHERE form_id = $1 AND is_active = false)
    RETURNING *
),
reverted_update AS (
    UPDATE workflow_versions AS wv
    SET is_active = true
    FROM deactivated AS d
    WHERE wv.id = d.id
      AND NOT EXISTS (SELECT 1 FROM activated)
    RETURNING wv.*
),
reverted AS (
    SELECT *
    FROM reverted_update
    ORDER BY updated_at DESC
    LIMIT 1
)
SELECT * FROM activated
UNION ALL
SELECT * FROM reverted;