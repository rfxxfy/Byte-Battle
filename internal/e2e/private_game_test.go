package e2e_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGame_PrivateGame(t *testing.T) {
	token3 := authToken(t, "private-test-outsider@test.com")

	resp := doAuth(t, http.MethodPost, "/api/games", map[string]any{
		"problem_ids": []string{"test-problem"},
		"is_public":   false,
	}, token1)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var g gameResp
	decodeJSON(t, resp, &g)

	assert.False(t, g.Game.IsPublic)
	require.NotNil(t, g.Game.InviteToken, "creator should receive invite_token")
	inviteToken := *g.Game.InviteToken

	t.Run("not visible in listing for outsider", func(t *testing.T) {
		resp := doAuth(t, http.MethodGet, "/api/games?limit=100&offset=0", nil, token3)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		var list gamesListResp
		decodeJSON(t, resp, &list)
		for _, game := range list.Games {
			assert.NotEqual(t, g.Game.ID, game.ID, "private game should not appear in outsider listing")
		}
	})

	t.Run("GET /games/{id}: 403 for outsider", func(t *testing.T) {
		resp := doAuth(t, http.MethodGet, fmt.Sprintf("/api/games/%d", g.Game.ID), nil, token3)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
		assert.Equal(t, "NOT_PARTICIPANT", errCode(t, resp))
	})

	t.Run("GET /games/join/{token}: 200 without auth", func(t *testing.T) {
		resp := do(t, http.MethodGet, fmt.Sprintf("/api/games/join/%s", inviteToken), nil)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		var fetched gameResp
		decodeJSON(t, resp, &fetched)
		assert.Equal(t, g.Game.ID, fetched.Game.ID)
	})

	t.Run("POST /games/join/{token}: outsider becomes participant", func(t *testing.T) {
		resp := doAuth(t, http.MethodPost, fmt.Sprintf("/api/games/join/%s", inviteToken), nil, token3)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		var joined gameResp
		decodeJSON(t, resp, &joined)
		assert.Equal(t, g.Game.ID, joined.Game.ID)

		resp = doAuth(t, http.MethodGet, fmt.Sprintf("/api/games/%d", g.Game.ID), nil, token3)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()
	})

	t.Run("POST /games/join/{token}: 409 if already participant", func(t *testing.T) {
		resp := doAuth(t, http.MethodPost, fmt.Sprintf("/api/games/join/%s", inviteToken), nil, token3)
		assert.Equal(t, http.StatusConflict, resp.StatusCode)
		assert.Equal(t, "ALREADY_PARTICIPANT", errCode(t, resp))
	})

	t.Run("GET /games/{id}/solutions: 403 for outsider on private game", func(t *testing.T) {
		token4 := authToken(t, "private-test-outsider2@test.com")
		resp := doAuth(t, http.MethodGet, fmt.Sprintf("/api/games/%d/solutions", g.Game.ID), nil, token4)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
		assert.Equal(t, "NOT_PARTICIPANT", errCode(t, resp))
	})

	t.Run("invite_token visible to creator in listing", func(t *testing.T) {
		resp := doAuth(t, http.MethodGet, "/api/games?limit=100&offset=0", nil, token1)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		var list struct {
			Games []struct {
				ID          int     `json:"id"`
				InviteToken *string `json:"invite_token"`
			} `json:"games"`
		}
		decodeJSON(t, resp, &list)
		for _, game := range list.Games {
			if game.ID == g.Game.ID {
				assert.NotNil(t, game.InviteToken, "creator should see invite_token in listing")
			}
		}
	})
}
