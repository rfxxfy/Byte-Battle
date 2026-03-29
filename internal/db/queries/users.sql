-- name: GetUserByUsername :one
SELECT * FROM users WHERE username = $1 LIMIT 1;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1 LIMIT 1;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1 LIMIT 1;

-- name: CreateUser :one
INSERT INTO users (username, email, password_hash)
VALUES ($1, $2, $3)
RETURNING *;

-- name: CreateUserByEmail :one
INSERT INTO users (username, email)
VALUES ($1, $2)
RETURNING *;

-- name: UpdateUserName :one
UPDATE users SET name = $2 WHERE id = $1 RETURNING *;

-- name: SetEmailVerified :exec
UPDATE users SET email_verified = true WHERE id = $1;

-- name: GetUserStats :one
SELECT
    COUNT(*) FILTER (WHERE g.winner_id = @user_id AND g.is_solo = false)::int AS wins,
    COUNT(*)::int AS games_played,
    COALESCE(SUM(gp.current_problem_index), 0)::int AS problems_solved
FROM game_participants gp
JOIN games g ON g.id = gp.game_id
WHERE gp.user_id = @user_id
  AND g.status = 'finished';
