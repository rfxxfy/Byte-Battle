-- name: AddGameProblem :exec
INSERT INTO game_problems (game_id, problem_index, problem_id, problem_version_id)
VALUES ($1, $2, $3, $4);

-- name: GetGameProblemIDs :many
SELECT problem_id
FROM game_problems
WHERE game_id = $1
ORDER BY problem_index;

-- name: GetGameProblemIDsByGameIDs :many
SELECT game_id, problem_id
FROM game_problems
WHERE game_id = ANY($1::int[])
ORDER BY game_id, problem_index;

-- name: CountGameProblems :one
SELECT count(*)
FROM game_problems
WHERE game_id = $1;

-- name: GetGameProblemIDByIndex :one
SELECT problem_id
FROM game_problems
WHERE game_id = $1 AND problem_index = $2
LIMIT 1;

-- name: GetGameProblemByIndex :one
SELECT gp.problem_id, gp.problem_version_id, pv.artifact_path
FROM game_problems gp
JOIN problem_versions pv ON pv.id = gp.problem_version_id
WHERE gp.game_id = $1 AND gp.problem_index = $2
LIMIT 1;
