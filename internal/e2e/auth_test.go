package e2e_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	sqlcdb "bytebattle/internal/db/sqlc"

	"github.com/jackc/pgx/v5/pgtype"
)

func TestAuth_Enter_InvalidEmail(t *testing.T) {
	resp := do(t, http.MethodPost, "/auth/enter", map[string]any{"email": "not-an-email"})
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, "INVALID_EMAIL", errCode(t, resp))
}

func TestAuth_Confirm_InvalidCode(t *testing.T) {
	seedCode(t, "player1@test.com", "123456", 5*time.Minute)

	resp := do(t, http.MethodPost, "/auth/confirm", map[string]any{
		"email": "player1@test.com",
		"code":  "000000", // wrong
	})
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, "INVALID_CODE", errCode(t, resp))
}

func TestAuth_Confirm_ExpiredCode(t *testing.T) {
	seedCode(t, "player1@test.com", "123456", -1*time.Minute)

	resp := do(t, http.MethodPost, "/auth/confirm", map[string]any{
		"email": "player1@test.com",
		"code":  "123456",
	})
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, "INVALID_CODE", errCode(t, resp))
}

func TestAuth_Confirm_UnknownEmail(t *testing.T) {
	resp := do(t, http.MethodPost, "/auth/confirm", map[string]any{
		"email": "ghost@test.com",
		"code":  "123456",
	})
	// unknown email must return same error as wrong code — prevents email enumeration
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, "INVALID_CODE", errCode(t, resp))
}

func TestAuth_Confirm_TooManyAttempts(t *testing.T) {
	seedCode(t, "player2@test.com", "999999", 5*time.Minute)

	// first MaxAttempts (5) wrong attempts must each return INVALID_CODE
	for range 5 {
		resp := do(t, http.MethodPost, "/auth/confirm", map[string]any{
			"email": "player2@test.com",
			"code":  "000000",
		})
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
		assert.Equal(t, "INVALID_CODE", errCode(t, resp))
	}

	// next attempt — even correct code — must be rejected
	resp := do(t, http.MethodPost, "/auth/confirm", map[string]any{
		"email": "player2@test.com",
		"code":  "999999",
	})
	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
	assert.Equal(t, "TOO_MANY_ATTEMPTS", errCode(t, resp))
}

func TestAuth_Me(t *testing.T) {
	resp := doAuth(t, http.MethodGet, "/auth/me", nil, token1)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var body struct {
		UserId string `json:"user_id"`
	}
	decodeJSON(t, resp, &body)
	assert.Equal(t, user1ID.String(), body.UserId)
}

func TestAuth_Me_Unauthorized(t *testing.T) {
	resp := do(t, http.MethodGet, "/auth/me", nil)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Equal(t, "INVALID_TOKEN", errCode(t, resp))
}

func TestAuth_Logout(t *testing.T) {
	tok := authToken(t, "player1@test.com")

	resp := doAuth(t, http.MethodPost, "/auth/logout", nil, tok)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	resp = doAuth(t, http.MethodGet, "/auth/me", nil, tok)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Equal(t, "INVALID_TOKEN", errCode(t, resp))
}

// seedCode inserts a known verification code for the given email directly into the DB,
// bypassing the mailer. ttl can be negative to produce an already-expired code.
func seedCode(t *testing.T, email, code string, ttl time.Duration) {
	t.Helper()
	ctx := context.Background()
	q := sqlcdb.New(testPool)

	hash, err := bcrypt.GenerateFromPassword([]byte(code), 4)
	require.NoError(t, err)

	_, err = q.UpsertVerificationCode(ctx, sqlcdb.UpsertVerificationCodeParams{
		Email:     email,
		CodeHash:  string(hash),
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(ttl), Valid: true},
	})
	require.NoError(t, err)
}
