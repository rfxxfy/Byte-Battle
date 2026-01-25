-- name: UpsertVerificationCode :one
INSERT INTO verification_codes (email, code_hash, expires_at)
VALUES ($1, $2, $3)
ON CONFLICT (email) DO UPDATE
    SET code_hash  = EXCLUDED.code_hash,
        expires_at = EXCLUDED.expires_at,
        attempts   = 0
RETURNING *;

-- name: GetVerificationCode :one
SELECT * FROM verification_codes WHERE email = $1;

-- name: IncrementAttemptsIfBelowLimit :one
UPDATE verification_codes
SET attempts = attempts + 1
WHERE email = $1 AND attempts < $2
RETURNING *;

-- name: DeleteVerificationCode :exec
DELETE FROM verification_codes WHERE email = $1;
