-- name: CreateUnit :one
INSERT INTO units (name, description, metadata, type)
VALUES ($1, $2, $3, 'unit')
RETURNING *;

-- name: CreateOrg :one
INSERT INTO organizations (name, description, metadata, type, slug)
VALUES ($1, $2, $3, 'organization', $4)
RETURNING *;

-- name: GetUnitByID :one
SELECT * FROM units WHERE id = $1;

-- name: GetOrgByID :one
SELECT * FROM organizations WHERE id = $1;

-- name: GetOrgIDBySlug :one
SELECT id FROM organizations WHERE slug = $1;

-- name: UpdateOrg :one
UPDATE organizations
SET slug = $2, name = $3, description = $4, metadata = $5, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateUnit :one
UPDATE units
SET name = $2, description = $3, metadata = $4, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: AddParentChild :one
INSERT INTO parent_child (parent_id, child_id)
VALUES ($1, $2)
RETURNING *;

-- name: ListSubUnits :many
SELECT u.* FROM units u
JOIN parent_child pc ON u.id = pc.child_id
WHERE pc.parent_id = $1;

-- name: ListSubUnitIDs :many
SELECT child_id FROM parent_child WHERE parent_id = $1;