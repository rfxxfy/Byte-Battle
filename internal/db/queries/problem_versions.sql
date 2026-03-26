-- name: CountProblemVersions :one
SELECT COUNT(*) FROM problem_versions WHERE problem_id = $1;

-- name: GetMaxProblemVersion :one
SELECT COALESCE(MAX(version), 0)::int FROM problem_versions WHERE problem_id = $1;

-- name: CreateProblemVersion :one
INSERT INTO problem_versions (
    problem_id,
    version,
    artifact_path,
    artifact_sha256,
    limits_time_ms,
    limits_memory_kb,
    checker_type,
    reference_language,
    created_by_user_id,
    test_case_count,
    difficulty
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;
