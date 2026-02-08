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
