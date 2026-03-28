-- name: AddGameProblem :exec
INSERT INTO game_problems (game_id, problem_index, problem_id)
VALUES ($1, $2, $3);

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

-- name: AdvanceGameProblem :one
UPDATE games
SET current_problem_index = $2,
    updated_at = NOW()
WHERE id = $1
RETURNING *;
