-- name: AddGameParticipant :exec
INSERT INTO game_participants (game_id, user_id)
VALUES ($1, $2);

-- name: IsGameParticipant :one
SELECT EXISTS(
    SELECT 1 FROM game_participants
    WHERE game_id = $1 AND user_id = $2
) AS is_participant;

-- name: CountGameParticipants :one
SELECT count(*) FROM game_participants WHERE game_id = $1;

-- name: RemoveGameParticipant :execrows
DELETE FROM game_participants WHERE game_id = $1 AND user_id = $2;

-- name: GetParticipants :many
SELECT gp.user_id, u.name
FROM game_participants gp
JOIN users u ON u.id = gp.user_id
WHERE gp.game_id = $1
ORDER BY gp.id;

-- name: GetParticipantsByGameIDs :many
SELECT gp.game_id, gp.user_id, u.name
FROM game_participants gp
JOIN users u ON u.id = gp.user_id
WHERE gp.game_id = ANY($1::int[])
ORDER BY gp.game_id, gp.id;

-- name: GetParticipantProblemIndex :one
SELECT current_problem_index FROM game_participants
WHERE game_id = $1 AND user_id = $2;

-- name: AdvanceParticipantProblem :one
UPDATE game_participants
SET current_problem_index = current_problem_index + 1
WHERE game_id = $1 AND user_id = $2
RETURNING current_problem_index;

-- name: GetAllParticipantsProblemIndices :many
SELECT user_id, current_problem_index FROM game_participants
WHERE game_id = $1;
