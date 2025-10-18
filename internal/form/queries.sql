-- name: Create :one
WITH created AS (
    INSERT INTO forms (title, description, preview_message, unit_id, last_editor, deadline)
    VALUES ($1, $2, $3, $4, $5, $6)
    RETURNING *
)
SELECT 
    f.*,
    u.name as unit_name,
    o.name as org_name,
    usr.name as last_editor_name,
    usr.username as last_editor_username,
    usr.avatar_url as last_editor_avatar_url,
    COALESCE(array_agg(e.value) FILTER (WHERE e.value IS NOT NULL), ARRAY[]::text[]) as last_editor_email
FROM created f
LEFT JOIN units u ON f.unit_id = u.id
LEFT JOIN units o ON u.org_id = o.id
LEFT JOIN users usr ON f.last_editor = usr.id
LEFT JOIN user_emails e ON usr.id = e.user_id
GROUP BY f.id, f.title, f.description, f.preview_message, f.status, f.unit_id, f.last_editor, f.deadline, f.created_at, f.updated_at, u.name, o.name, usr.name, usr.username, usr.avatar_url;  

-- name: Update :one
WITH updated AS (
    UPDATE forms
    SET title = $2, description = $3, preview_message = $4, last_editor = $5, deadline = $6, updated_at = now()
    WHERE forms.id = $1
    RETURNING *
)
SELECT 
    f.*,
    u.name as unit_name,
    o.name as org_name,
    usr.name as last_editor_name,
    usr.username as last_editor_username,
    usr.avatar_url as last_editor_avatar_url,
    COALESCE(array_agg(e.value) FILTER (WHERE e.value IS NOT NULL), ARRAY[]::text[]) as last_editor_email
FROM updated f
LEFT JOIN units u ON f.unit_id = u.id
LEFT JOIN units o ON u.org_id = o.id
LEFT JOIN users usr ON f.last_editor = usr.id
LEFT JOIN user_emails e ON usr.id = e.user_id
GROUP BY f.id, f.title, f.description, f.preview_message, f.status, f.unit_id, f.last_editor, f.deadline, f.created_at, f.updated_at, u.name, o.name, usr.name, usr.username, usr.avatar_url;

-- name: Delete :execrows
DELETE FROM forms WHERE id = $1;

-- name: GetByID :one
SELECT 
    f.*,
    u.name as unit_name,
    o.name as org_name,
    usr.name as last_editor_name,
    usr.username as last_editor_username,
    usr.avatar_url as last_editor_avatar_url,
    COALESCE(array_agg(e.value) FILTER (WHERE e.value IS NOT NULL), ARRAY[]::text[]) as last_editor_email
FROM forms f
LEFT JOIN units u ON f.unit_id = u.id
LEFT JOIN units o ON u.org_id = o.id
LEFT JOIN users usr ON f.last_editor = usr.id
LEFT JOIN user_emails e ON usr.id = e.user_id
WHERE f.id = $1
GROUP BY f.id, f.title, f.description, f.preview_message, f.status, f.unit_id, f.last_editor, f.deadline, f.created_at, f.updated_at, u.name, o.name, usr.name, usr.username, usr.avatar_url;

-- name: List :many
SELECT 
    f.*,
    u.name as unit_name,
    o.name as org_name,
    usr.name as last_editor_name,
    usr.username as last_editor_username,
    usr.avatar_url as last_editor_avatar_url,
    COALESCE(array_agg(e.value) FILTER (WHERE e.value IS NOT NULL), ARRAY[]::text[]) as last_editor_email
FROM forms f
LEFT JOIN units u ON f.unit_id = u.id
LEFT JOIN units o ON u.org_id = o.id
LEFT JOIN users usr ON f.last_editor = usr.id
LEFT JOIN user_emails e ON usr.id = e.user_id
GROUP BY f.id, f.title, f.description, f.preview_message, f.status, f.unit_id, f.last_editor, f.deadline, f.created_at, f.updated_at, u.name, o.name, usr.name, usr.username, usr.avatar_url
ORDER BY f.updated_at DESC;

-- name: ListByUnit :many
SELECT 
    f.*,
    u.name as unit_name,
    o.name as org_name,
    usr.name as last_editor_name,
    usr.username as last_editor_username,
    usr.avatar_url as last_editor_avatar_url,
    COALESCE(array_agg(e.value) FILTER (WHERE e.value IS NOT NULL), ARRAY[]::text[]) as last_editor_email
FROM forms f
LEFT JOIN units u ON f.unit_id = u.id
LEFT JOIN units o ON u.org_id = o.id
LEFT JOIN users usr ON f.last_editor = usr.id
LEFT JOIN user_emails e ON usr.id = e.user_id
WHERE f.unit_id = $1
GROUP BY f.id, f.title, f.description, f.preview_message, f.status, f.unit_id, f.last_editor, f.deadline, f.created_at, f.updated_at, u.name, o.name, usr.name, usr.username, usr.avatar_url
ORDER BY f.updated_at DESC;

-- name: SetStatus :one
UPDATE forms
SET status = $2, last_editor = $3, updated_at = now()
WHERE id = $1
RETURNING *;