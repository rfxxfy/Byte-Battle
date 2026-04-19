-- name: GetProblemCatalogBySlug :one
SELECT *
FROM problems
WHERE slug = $1
LIMIT 1;

-- name: CreateProblemCatalog :one
INSERT INTO problems (slug, owner_user_id, visibility, status, title)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: SetProblemCurrentVersion :exec
UPDATE problems
SET current_version_id = $2,
    updated_at = NOW()
WHERE id = $1;

-- name: ListPublishedPublicProblems :many
SELECT *
FROM problems
WHERE status = 'published' AND visibility = 'public'
ORDER BY created_at DESC;
