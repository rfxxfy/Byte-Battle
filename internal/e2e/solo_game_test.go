package e2e_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGame_Solo(t *testing.T) {
	t.Run("create solo game, start with 1 participant", func(t *testing.T) {
		resp := doAuth(t, http.MethodPost, "/api/games", map[string]any{
			"problem_ids": []string{"test-problem"},
			"is_solo":     true,
		}, token1)
		require.Equal(t, http.StatusCreated, resp.StatusCode)
		var g gameResp
		decodeJSON(t, resp, &g)

		assert.True(t, g.Game.IsSolo)
		assert.False(t, g.Game.IsPublic)
		assert.Equal(t, "pending", g.Game.Status)

		resp = doAuth(t, http.MethodPost, fmt.Sprintf("/api/games/%d/start", g.Game.ID), nil, token1)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		decodeJSON(t, resp, &g)
		assert.Equal(t, "active", g.Game.Status)
	})

	t.Run("solo game with time limit", func(t *testing.T) {
		resp := doAuth(t, http.MethodPost, "/api/games", map[string]any{
			"problem_ids":        []string{"test-problem"},
			"is_solo":            true,
			"time_limit_minutes": 30,
		}, token1)
		require.Equal(t, http.StatusCreated, resp.StatusCode)
		var g gameResp
		decodeJSON(t, resp, &g)
		require.NotNil(t, g.Game.TimeLimitMinutes)
		assert.Equal(t, 30, *g.Game.TimeLimitMinutes)
	})

	t.Run("timeout finishes active solo game with no winner", func(t *testing.T) {
		resp := doAuth(t, http.MethodPost, "/api/games", map[string]any{
			"problem_ids":        []string{"test-problem"},
			"is_solo":            true,
			"time_limit_minutes": 15,
		}, token1)
		require.Equal(t, http.StatusCreated, resp.StatusCode)
		var g gameResp
		decodeJSON(t, resp, &g)

		resp = doAuth(t, http.MethodPost, fmt.Sprintf("/api/games/%d/start", g.Game.ID), nil, token1)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()

		resp = doAuth(t, http.MethodPost, fmt.Sprintf("/api/games/%d/timeout", g.Game.ID), nil, token1)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		decodeJSON(t, resp, &g)
		assert.Equal(t, "finished", g.Game.Status)
		assert.Nil(t, g.Game.WinnerID)
	})

	t.Run("timeout not allowed on multiplayer game", func(t *testing.T) {
		g := createGame(t)
		resp := doAuth(t, http.MethodPost, fmt.Sprintf("/api/games/%d/start", g.Game.ID), nil, token1)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()

		resp = doAuth(t, http.MethodPost, fmt.Sprintf("/api/games/%d/timeout", g.Game.ID), nil, token1)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
		assert.Equal(t, "VALIDATION_ERROR", errCode(t, resp))
	})

	t.Run("solo not visible to other users in listing", func(t *testing.T) {
		resp := doAuth(t, http.MethodPost, "/api/games", map[string]any{
			"problem_ids": []string{"test-problem"},
			"is_solo":     true,
		}, token1)
		require.Equal(t, http.StatusCreated, resp.StatusCode)
		var g gameResp
		decodeJSON(t, resp, &g)

		resp = doAuth(t, http.MethodGet, "/api/games?limit=100&offset=0", nil, token2)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		var list gamesListResp
		decodeJSON(t, resp, &list)
		for _, game := range list.Games {
			assert.NotEqual(t, g.Game.ID, game.ID, "solo game should not appear in other user's listing")
		}
	})
}
