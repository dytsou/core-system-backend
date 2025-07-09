-- name: CreateUnit :one
INSERT INTO units (id, name, description, metadata)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: CreateOrg :one
INSERT INTO organizations (id, name, description, metadata, slug)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetUnitByID :one
SELECT * FROM units WHERE id = $1;

-- name: GetOrgByID :one
SELECT * FROM organizations WHERE id = $1;

-- name: GetOrgIDBySlug :one
SELECT id FROM organizations WHERE slug = $1;