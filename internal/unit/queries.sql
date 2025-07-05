-- name: CreateUnit :one
INSERT INTO units (id, name, description, metadata, type)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: CreateOrg :one
INSERT INTO organizations (id, name, description, metadata, type, slug)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetUnitByID :one
SELECT * FROM units WHERE id = $1;

-- name: GetOrgByID :one
SELECT * FROM organizations WHERE id = $1;

-- name: ListSubUnits :many
SELECT u.* FROM units u
JOIN parent_child pc ON pc.child_id = u.id
WHERE pc.parent_id = $1;

-- name: UpdateUnit :one
UPDATE units
SET name = $2, description = $3, metadata = $4, type = $5, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateOrg :one
UPDATE organizations
SET name = $2, description = $3, metadata = $4, type = $5, slug = $6, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteUnit :exec
DELETE FROM units WHERE id = $1;

-- name: DeleteOrg :exec
DELETE FROM organizations WHERE id = $1;

-- name: AddParentChild :one
INSERT INTO parent_child (parent_id, child_id)
VALUES ($1, $2)
RETURNING *;

-- name: DeleteParentChild :exec
DELETE FROM parent_child WHERE parent_id = $1 AND child_id = $2;

-- name: ListSubUnitIDs :many
SELECT child_id FROM parent_child WHERE parent_id = $1;
