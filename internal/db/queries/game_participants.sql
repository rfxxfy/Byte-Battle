-- name: AddGameParticipant :exec
INSERT INTO game_participants (game_id, user_id)
VALUES ($1, $2);

-- name: IsGameParticipant :one
SELECT EXISTS(
    SELECT 1 FROM game_participants
    WHERE game_id = $1 AND user_id = $2
) AS is_participant;

-- name: GetParticipantIDs :many
SELECT user_id FROM game_participants WHERE game_id = $1 ORDER BY id;

-- name: CountGameParticipants :one
SELECT count(*) FROM game_participants WHERE game_id = $1;
