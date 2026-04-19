-- name: CreateProblemVersion :one
INSERT INTO problem_versions (
    problem_id,
    version,
    artifact_path,
    artifact_sha256,
    statement_sha256,
    limits_time_ms,
    limits_memory_kb,
    checker_type,
    created_by_user_id
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;
