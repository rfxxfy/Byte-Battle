-- name: CreateSession :one
INSERT INTO sessions (user_id, token, expires_at)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetSessionByID :one
SELECT * FROM sessions WHERE id = $1 LIMIT 1;

-- name: GetSessionByToken :one
SELECT * FROM sessions WHERE token = $1 LIMIT 1;

-- name: GetSessionsByUserID :many
SELECT * FROM sessions WHERE user_id = $1 ORDER BY created_at DESC;

-- name: UpdateSessionExpiry :one
UPDATE sessions
SET expires_at = $2,
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteSession :execrows
DELETE FROM sessions WHERE id = $1;

-- name: DeleteExpiredSessions :execrows
DELETE FROM sessions WHERE expires_at < NOW();

-- name: DeleteSessionsByUserID :execrows
DELETE FROM sessions WHERE user_id = $1;
