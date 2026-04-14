-- name: CreateGame :one
INSERT INTO games (creator_id, status, is_public)
VALUES ($1, 'pending', $2)
RETURNING *;

-- name: GetGameByID :one
SELECT * FROM games WHERE id = $1 LIMIT 1;

-- name: GetGameForUpdate :one
SELECT * FROM games WHERE id = $1 LIMIT 1 FOR UPDATE;

-- name: GetGameByInviteToken :one
SELECT * FROM games WHERE invite_token = $1 LIMIT 1;

-- name: ListGamesForUser :many
SELECT * FROM games
WHERE is_public = true
   OR creator_id = sqlc.arg(user_id)::uuid
   OR EXISTS (
       SELECT 1 FROM game_participants
       WHERE game_id = games.id AND user_id = sqlc.arg(user_id)::uuid
   )
ORDER BY created_at DESC LIMIT $1 OFFSET $2;

-- name: CountGamesForUser :one
SELECT count(*) FROM games
WHERE is_public = true
   OR creator_id = sqlc.arg(user_id)::uuid
   OR EXISTS (
       SELECT 1 FROM game_participants
       WHERE game_id = games.id AND user_id = sqlc.arg(user_id)::uuid
   );

-- name: StartGame :one
UPDATE games
SET status = 'active',
    started_at = NOW(),
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: CompleteGame :one
UPDATE games
SET status = 'finished',
    winner_id = $2,
    completed_at = NOW(),
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: CancelGame :one
UPDATE games
SET status = 'cancelled',
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: UpdateGameWinner :exec
UPDATE games SET winner_id = $2, updated_at = NOW() WHERE id = $1;

-- name: DeleteGame :execrows
DELETE FROM games WHERE id = $1;
