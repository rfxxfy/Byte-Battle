package e2e_test

import (
	"context"
	"net/http"
	"strings"
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

func TestAuth_PatchMe_SetName(t *testing.T) {
	tok := authToken(t, "player1@test.com")

	resp := doAuth(t, http.MethodPatch, "/auth/me", map[string]any{"name": "Alice"}, tok)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var patchBody struct {
		Name *string `json:"name"`
	}
	decodeJSON(t, resp, &patchBody)
	require.NotNil(t, patchBody.Name)
	assert.Equal(t, "Alice", *patchBody.Name)

	// GetAuthMe should now return the name
	resp2 := doAuth(t, http.MethodGet, "/auth/me", nil, tok)
	require.Equal(t, http.StatusOK, resp2.StatusCode)
	var meBody struct {
		Name *string `json:"name"`
	}
	decodeJSON(t, resp2, &meBody)
	require.NotNil(t, meBody.Name)
	assert.Equal(t, "Alice", *meBody.Name)
}

func TestAuth_PatchMe_TrimsWhitespace(t *testing.T) {
	tok := authToken(t, "player1@test.com")

	resp := doAuth(t, http.MethodPatch, "/auth/me", map[string]any{"name": "  Bob  "}, tok)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var body struct {
		Name *string `json:"name"`
	}
	decodeJSON(t, resp, &body)
	require.NotNil(t, body.Name)
	assert.Equal(t, "Bob", *body.Name)
}

func TestAuth_PatchMe_EmptyName(t *testing.T) {
	resp := doAuth(t, http.MethodPatch, "/auth/me", map[string]any{"name": ""}, token1)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, "VALIDATION_ERROR", errCode(t, resp))
}

func TestAuth_PatchMe_WhitespaceOnlyName(t *testing.T) {
	resp := doAuth(t, http.MethodPatch, "/auth/me", map[string]any{"name": "   "}, token1)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, "VALIDATION_ERROR", errCode(t, resp))
}

func TestAuth_PatchMe_TooLongName(t *testing.T) {
	resp := doAuth(t, http.MethodPatch, "/auth/me", map[string]any{"name": strings.Repeat("a", 101)}, token1)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, "VALIDATION_ERROR", errCode(t, resp))
}

func TestAuth_PatchMe_Unauthorized(t *testing.T) {
	resp := do(t, http.MethodPatch, "/auth/me", map[string]any{"name": "Alice"})
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Equal(t, "INVALID_TOKEN", errCode(t, resp))
}

func TestAuth_Confirm_IncludesName(t *testing.T) {
	// Give player2 a known name
	tok := authToken(t, "player2@test.com")
	resp := doAuth(t, http.MethodPatch, "/auth/me", map[string]any{"name": "Player Two"}, tok)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// New login — confirm response should include the name without extra /auth/me call
	seedCode(t, "player2@test.com", "111111", 5*time.Minute)
	resp2 := do(t, http.MethodPost, "/auth/confirm", map[string]any{
		"email": "player2@test.com",
		"code":  "111111",
	})
	require.Equal(t, http.StatusOK, resp2.StatusCode)
	var body struct {
		Token string  `json:"token"`
		Name  *string `json:"name"`
	}
	decodeJSON(t, resp2, &body)
	require.NotNil(t, body.Name)
	assert.Equal(t, "Player Two", *body.Name)
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
