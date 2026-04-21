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

-- name: ListPublishedPublicProblemsWithArtifact :many
SELECT p.id, p.slug, p.owner_user_id, p.visibility, p.status, p.title, p.current_version_id, p.created_at, p.updated_at, pv.artifact_path
FROM problems p
JOIN problem_versions pv ON pv.id = p.current_version_id
WHERE p.status = 'published' AND p.visibility = 'public'
ORDER BY p.created_at DESC;

-- name: GetProblemWithArtifactBySlug :one
SELECT p.id, p.slug, p.owner_user_id, p.visibility, p.status, p.title, p.current_version_id, p.created_at, p.updated_at, pv.artifact_path
FROM problems p
JOIN problem_versions pv ON pv.id = p.current_version_id
WHERE p.slug = $1
LIMIT 1;

-- name: ListPublicProblemsSearch :many
SELECT p.id, p.slug, p.title, p.visibility, p.current_version_id, p.owner_user_id, pv.artifact_path
FROM problems p
JOIN problem_versions pv ON pv.id = p.current_version_id
WHERE p.status = 'published' AND p.visibility = 'public'
  AND ($3::text = '' OR p.title ILIKE '%' || $3::text || '%' OR p.slug ILIKE '%' || $3::text || '%')
ORDER BY p.created_at DESC
LIMIT $1 OFFSET $2;

-- name: CountPublicProblems :one
SELECT COUNT(*)
FROM problems
WHERE status = 'published' AND visibility = 'public'
  AND ($1::text = '' OR title ILIKE '%' || $1::text || '%' OR slug ILIKE '%' || $1::text || '%');

-- name: ListMyProblems :many
SELECT p.id, p.slug, p.title, p.visibility, p.status, p.current_version_id, p.owner_user_id,
       p.created_at, p.updated_at, pv.artifact_path, pv.version
FROM problems p
LEFT JOIN problem_versions pv ON pv.id = p.current_version_id
WHERE p.owner_user_id = $1
  AND ($2::text = '' OR p.title ILIKE '%' || $2::text || '%' OR p.slug ILIKE '%' || $2::text || '%')
ORDER BY p.created_at DESC;

-- name: CountUserProblems :one
SELECT COUNT(*) FROM problems WHERE owner_user_id = $1;

-- name: UpdateProblemVisibility :exec
UPDATE problems SET visibility = $2, updated_at = NOW() WHERE id = $1;

-- name: LockProblemForUpdate :one
SELECT id FROM problems WHERE id = $1 FOR UPDATE;
