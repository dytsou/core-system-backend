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

-- name: AddParentChild :one
INSERT INTO parent_child (parent_id, child_id)
VALUES ($1, $2)
RETURNING *;

-- name: ListSubUnits :many
SELECT u.* FROM units u
JOIN parent_child pc ON u.id = pc.child_id
WHERE pc.parent_id = $1;