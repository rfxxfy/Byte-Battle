-- name: UpsertVerificationCode :one
INSERT INTO email_verification_codes (user_id, code_hash, expires_at)
VALUES ($1, $2, $3)
ON CONFLICT (user_id) DO UPDATE
    SET code_hash  = EXCLUDED.code_hash,
        expires_at = EXCLUDED.expires_at,
        attempts   = 0
RETURNING *;

-- name: GetVerificationCodeByUserID :one
SELECT * FROM email_verification_codes WHERE user_id = $1 LIMIT 1;

-- name: IncrementAttemptsIfBelowLimit :one
UPDATE email_verification_codes
SET attempts = attempts + 1
WHERE id = $1 AND attempts < $2
RETURNING id, user_id, code_hash, expires_at, attempts, created_at;

-- name: DeleteVerificationCode :exec
DELETE FROM email_verification_codes WHERE id = $1;
