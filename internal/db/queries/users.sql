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
