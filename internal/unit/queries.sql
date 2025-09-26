-- name: Create :one
INSERT INTO units (name, org_id, description, metadata, type, parent_id)
VALUES ($1, $2, $3, $4, $5, $6)
    RETURNING *;

-- name: GetByID :one
SELECT * FROM units WHERE id = $1;

-- name: GetAllOrganizations :many
SELECT * FROM units WHERE type = 'organization';

-- name: Update :one
UPDATE units
SET name = $2,
    description = $3,
    metadata = $4,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateParent :one
UPDATE units
SET parent_id = $2,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: Delete :exec
DELETE FROM units WHERE id = $1;

-- name: ListSubUnits :many
SELECT * FROM units WHERE parent_id = $1;

-- name: ListSubUnitIDs :many
SELECT id FROM units WHERE parent_id = $1;

-- name: AddMember :one
INSERT INTO unit_members (unit_id, member_id)
VALUES ($1, $2)
    RETURNING *;

-- name: ListMembers :many
SELECT member_id FROM unit_members WHERE unit_id = $1;

-- name: ListUnitsMembers :many
SELECT unit_id, member_id
FROM unit_members
WHERE unit_id = ANY($1::uuid[]);

-- name: RemoveMember :exec
DELETE FROM unit_members WHERE unit_id = $1 AND member_id = $2;
