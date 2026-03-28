-- name: CreateGame :one
INSERT INTO games (creator_id, status)
VALUES ($1, 'pending')
RETURNING *;

-- name: GetGameByID :one
SELECT * FROM games WHERE id = $1 LIMIT 1;

-- name: GetGameForUpdate :one
SELECT * FROM games WHERE id = $1 LIMIT 1 FOR UPDATE;

-- name: ListGames :many
SELECT * FROM games ORDER BY created_at DESC LIMIT $1 OFFSET $2;

-- name: CountGames :one
SELECT count(*) FROM games;

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

-- name: DeleteGame :execrows
DELETE FROM games WHERE id = $1;
