package e2e_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"

	"bytebattle/internal/service"
)

func TestExecute_Auth(t *testing.T) {
	// Use a dedicated server with unlimited rate to avoid burst exhaustion
	// when the test runs multiple times (e.g. -count=10).
	srv := newGameServer(t, correctExecutor{}, service.RateLimitConfig{Rate: rate.Inf, Burst: 1000})

	execute := func(auth string) *http.Response {
		req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/execute",
			strings.NewReader(`{"code":"x","language":"go","input":""}`))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		if auth != "" {
			req.Header.Set("Authorization", auth)
		}
		resp, err := srv.Client().Do(req)
		require.NoError(t, err)
		return resp
	}

	t.Run("no auth header", func(t *testing.T) {
		resp := execute("")
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		assert.Equal(t, "INVALID_TOKEN", errCode(t, resp))
	})

	t.Run("invalid scheme", func(t *testing.T) {
		resp := execute("Token abc123")
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		assert.Equal(t, "INVALID_TOKEN", errCode(t, resp))
	})

	t.Run("nonexistent bearer token", func(t *testing.T) {
		resp := execute("Bearer doesnotexist")
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		assert.Equal(t, "INVALID_TOKEN", errCode(t, resp))
	})

	t.Run("valid token", func(t *testing.T) {
		resp := execute("Bearer " + token1)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()
	})
}
