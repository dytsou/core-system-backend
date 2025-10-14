-- name: ExistsBySlug :one
SELECT EXISTS(SELECT 1 FROM tenants WHERE slug = $1);

-- name: Create :one
INSERT INTO tenants (id, slug, db_strategy, owner_id)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: Get :one
SELECT * FROM tenants WHERE id = $1;

-- name: GetBySlug :one
SELECT * FROM tenants WHERE slug = $1;

-- name: Update :one
UPDATE tenants
SET slug = $2, db_strategy = $3
WHERE id = $1
RETURNING *;

-- name: Delete :exec
DELETE FROM tenants
WHERE id = $1;

-- name: GetSlugHistory :many
SELECT * FROM slug_history WHERE slug = $1;

-- name: CreateSlugHistory :one
INSERT INTO slug_history (slug, org_id, orgName, created_at, ended_at)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: UpdateSlugHistory :one
UPDATE slug_history
SET ended_at = $3
WHERE slug = $1 AND org_id = $2
RETURNING *;