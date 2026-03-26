package e2e_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecute_Auth(t *testing.T) {
	execute := func(auth string) *http.Response {
		req, err := http.NewRequest(http.MethodPost, testSrv.URL+"/api/execute",
			strings.NewReader(`{"code":"x","language":"go","input":""}`))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		if auth != "" {
			req.Header.Set("Authorization", auth)
		}
		resp, err := testSrv.Client().Do(req)
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
