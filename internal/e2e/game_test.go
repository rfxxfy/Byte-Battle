package e2e_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type gameResp struct {
	Game struct {
		ID           int     `json:"id"`
		ProblemID    string  `json:"problem_id"`
		CreatorID    string  `json:"creator_id"`
		Status       string  `json:"status"`
		WinnerID     *string `json:"winner_id"`
		Participants []struct {
			ID   string  `json:"id"`
			Name *string `json:"name"`
		} `json:"participants"`
	} `json:"game"`
}

func participantIDs(g gameResp) []string {
	ids := make([]string, len(g.Game.Participants))
	for i, p := range g.Game.Participants {
		ids[i] = p.ID
	}
	return ids
}

type gamesListResp struct {
	Games []struct {
		ID     int    `json:"id"`
		Status string `json:"status"`
	} `json:"games"`
	Total int64 `json:"total"`
}

// createGame creates a pending game as user1, then user2 joins.
func createGame(t *testing.T) gameResp {
	t.Helper()
	resp := doAuth(t, http.MethodPost, "/api/games", map[string]any{
		"problem_id": "test-problem",
	}, token1)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var g gameResp
	decodeJSON(t, resp, &g)

	resp = doAuth(t, http.MethodPost, fmt.Sprintf("/api/games/%d/join", g.Game.ID), nil, token2)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	decodeJSON(t, resp, &g)
	return g
}

func createActiveGame(t *testing.T) gameResp {
	t.Helper()
	g := createGame(t)
	resp := doAuth(t, http.MethodPost, fmt.Sprintf("/api/games/%d/start", g.Game.ID), nil, token1)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var started gameResp
	decodeJSON(t, resp, &started)
	return started
}

func TestGame_CreateAndGet(t *testing.T) {
	g := createGame(t)
	assert.Equal(t, "pending", g.Game.Status)
	assert.Equal(t, "test-problem", g.Game.ProblemID)
	assert.Equal(t, user1ID.String(), g.Game.CreatorID)
	assert.ElementsMatch(t, []string{user1ID.String(), user2ID.String()}, participantIDs(g))

	resp := doAuth(t, http.MethodGet, fmt.Sprintf("/api/games/%d", g.Game.ID), nil, token1)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	var fetched gameResp
	decodeJSON(t, resp, &fetched)
	assert.Equal(t, g.Game.ID, fetched.Game.ID)
	assert.ElementsMatch(t, []string{user1ID.String(), user2ID.String()}, participantIDs(fetched))
}

func TestGame_JoinValidation(t *testing.T) {
	t.Run("already a participant", func(t *testing.T) {
		resp := doAuth(t, http.MethodPost, "/api/games", map[string]any{
			"problem_id": "test-problem",
		}, token1)
		require.Equal(t, http.StatusCreated, resp.StatusCode)
		var g gameResp
		decodeJSON(t, resp, &g)

		// user1 tries to join their own game
		resp = doAuth(t, http.MethodPost, fmt.Sprintf("/api/games/%d/join", g.Game.ID), nil, token1)
		assert.Equal(t, http.StatusConflict, resp.StatusCode)
		assert.Equal(t, "ALREADY_PARTICIPANT", errCode(t, resp))
	})

	t.Run("join non-pending game", func(t *testing.T) {
		g := createActiveGame(t)

		token3 := authToken(t, "player3@test.com")
		resp := doAuth(t, http.MethodPost, fmt.Sprintf("/api/games/%d/join", g.Game.ID), nil, token3)
		assert.Equal(t, http.StatusConflict, resp.StatusCode)
		assert.Equal(t, "GAME_ALREADY_STARTED", errCode(t, resp))
	})

	t.Run("third player can join pending game", func(t *testing.T) {
		g := createGame(t)

		token3 := authToken(t, "player3-full@test.com")
		resp := doAuth(t, http.MethodPost, fmt.Sprintf("/api/games/%d/join", g.Game.ID), nil, token3)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		var joined gameResp
		decodeJSON(t, resp, &joined)
		assert.Len(t, joined.Game.Participants, 3)
	})
}

func TestGame_StartValidation(t *testing.T) {
	t.Run("not enough players", func(t *testing.T) {
		// Create game without user2 joining
		resp := doAuth(t, http.MethodPost, "/api/games", map[string]any{
			"problem_id": "test-problem",
		}, token1)
		require.Equal(t, http.StatusCreated, resp.StatusCode)
		var g gameResp
		decodeJSON(t, resp, &g)

		resp = doAuth(t, http.MethodPost, fmt.Sprintf("/api/games/%d/start", g.Game.ID), nil, token1)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
		assert.Equal(t, "NOT_ENOUGH_PLAYERS", errCode(t, resp))
	})

	t.Run("non-creator cannot start", func(t *testing.T) {
		g := createGame(t) // token1 created, token2 joined

		resp := doAuth(t, http.MethodPost, fmt.Sprintf("/api/games/%d/start", g.Game.ID), nil, token2)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
		assert.Equal(t, "NOT_GAME_CREATOR", errCode(t, resp))
	})
}

func TestGame_NotFound(t *testing.T) {
	const nonexistent = 999999

	resp := doAuth(t, http.MethodGet, fmt.Sprintf("/api/games/%d", nonexistent), nil, token1)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	assert.Equal(t, "GAME_NOT_FOUND", errCode(t, resp))

	resp = doAuth(t, http.MethodPost, fmt.Sprintf("/api/games/%d/start", nonexistent), nil, token1)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()

	resp = doAuth(t, http.MethodPost, fmt.Sprintf("/api/games/%d/cancel", nonexistent), nil, token1)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()

	resp = doAuth(t, http.MethodDelete, fmt.Sprintf("/api/games/%d", nonexistent), nil, token1)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()
}

func TestGame_List(t *testing.T) {
	for range 2 {
		createGame(t)
	}

	// one extra pending game (without join) to increase total
	resp := doAuth(t, http.MethodPost, "/api/games", map[string]any{
		"problem_id": "test-problem",
	}, token1)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()
	resp = doAuth(t, http.MethodGet, "/api/games?limit=2&offset=0", nil, token1)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var list gamesListResp
	decodeJSON(t, resp, &list)
	assert.Len(t, list.Games, 2)
	assert.GreaterOrEqual(t, list.Total, int64(3))
}

func TestGame_Delete(t *testing.T) {
	g := createGame(t)

	resp := doAuth(t, http.MethodDelete, fmt.Sprintf("/api/games/%d", g.Game.ID), nil, token1)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	resp = doAuth(t, http.MethodGet, fmt.Sprintf("/api/games/%d", g.Game.ID), nil, token1)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()
}

func TestGame_FullLifecycle(t *testing.T) {
	g := createGame(t)
	assert.Equal(t, "pending", g.Game.Status)

	resp := doAuth(t, http.MethodPost, fmt.Sprintf("/api/games/%d/start", g.Game.ID), nil, token1)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var started gameResp
	decodeJSON(t, resp, &started)
	assert.Equal(t, "active", started.Game.Status)

	resp = doAuth(t, http.MethodPost, fmt.Sprintf("/api/games/%d/complete", g.Game.ID), map[string]any{
		"winner_id": user1ID.String(),
	}, token1)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var completed gameResp
	decodeJSON(t, resp, &completed)
	assert.Equal(t, "finished", completed.Game.Status)
	require.NotNil(t, completed.Game.WinnerID)
	assert.Equal(t, user1ID.String(), *completed.Game.WinnerID)
}

func TestGame_CancelPending(t *testing.T) {
	g := createGame(t)

	resp := doAuth(t, http.MethodPost, fmt.Sprintf("/api/games/%d/cancel", g.Game.ID), nil, token1)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var cancelled gameResp
	decodeJSON(t, resp, &cancelled)
	assert.Equal(t, "cancelled", cancelled.Game.Status)
}

func TestGame_InvalidTransitions(t *testing.T) {
	t.Run("start already active", func(t *testing.T) {
		g := createGame(t)
		resp := doAuth(t, http.MethodPost, fmt.Sprintf("/api/games/%d/start", g.Game.ID), nil, token1)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()

		resp = doAuth(t, http.MethodPost, fmt.Sprintf("/api/games/%d/start", g.Game.ID), nil, token1)
		assert.Equal(t, http.StatusConflict, resp.StatusCode)
		assert.Equal(t, "GAME_ALREADY_STARTED", errCode(t, resp))
	})

	t.Run("complete pending game", func(t *testing.T) {
		g := createGame(t)
		resp := doAuth(t, http.MethodPost, fmt.Sprintf("/api/games/%d/complete", g.Game.ID), map[string]any{
			"winner_id": user1ID.String(),
		}, token1)
		assert.Equal(t, http.StatusConflict, resp.StatusCode)
		assert.Equal(t, "GAME_NOT_IN_PROGRESS", errCode(t, resp))
	})

	t.Run("cancel finished game", func(t *testing.T) {
		g := createGame(t)
		resp := doAuth(t, http.MethodPost, fmt.Sprintf("/api/games/%d/start", g.Game.ID), nil, token1)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()

		resp = doAuth(t, http.MethodPost, fmt.Sprintf("/api/games/%d/complete", g.Game.ID), map[string]any{
			"winner_id": user1ID.String(),
		}, token1)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()

		resp = doAuth(t, http.MethodPost, fmt.Sprintf("/api/games/%d/cancel", g.Game.ID), nil, token1)
		assert.Equal(t, http.StatusConflict, resp.StatusCode)
		assert.Equal(t, "CANNOT_CANCEL_FINISHED_GAME", errCode(t, resp))
	})

	t.Run("cancel already cancelled", func(t *testing.T) {
		g := createGame(t)
		resp := doAuth(t, http.MethodPost, fmt.Sprintf("/api/games/%d/cancel", g.Game.ID), nil, token1)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()

		resp = doAuth(t, http.MethodPost, fmt.Sprintf("/api/games/%d/cancel", g.Game.ID), nil, token1)
		assert.Equal(t, http.StatusConflict, resp.StatusCode)
		assert.Equal(t, "GAME_ALREADY_CANCELLED", errCode(t, resp))
	})

	t.Run("complete with non-participant winner", func(t *testing.T) {
		g := createGame(t)
		resp := doAuth(t, http.MethodPost, fmt.Sprintf("/api/games/%d/start", g.Game.ID), nil, token1)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()

		resp = doAuth(t, http.MethodPost, fmt.Sprintf("/api/games/%d/complete", g.Game.ID), map[string]any{
			"winner_id": "00000000-0000-0000-0000-000000009999",
		}, token1)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
		assert.Equal(t, "INVALID_WINNER", errCode(t, resp))
	})
}

func TestGame_CreateWithUnknownProblemID(t *testing.T) {
	resp := doAuth(t, http.MethodPost, "/api/games", map[string]any{
		"problem_id": "does-not-exist",
	}, token1)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	assert.Equal(t, "PROBLEM_NOT_FOUND", errCode(t, resp))
}

func TestGame_Leave(t *testing.T) {
	t.Run("participant leaves successfully", func(t *testing.T) {
		g := createGame(t) // user1 created, user2 joined
		assert.ElementsMatch(t, []string{user1ID.String(), user2ID.String()}, participantIDs(g))

		resp := doAuth(t, http.MethodPost, fmt.Sprintf("/api/games/%d/leave", g.Game.ID), nil, token2)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		var left gameResp
		decodeJSON(t, resp, &left)
		assert.Equal(t, "pending", left.Game.Status)
		assert.Equal(t, []string{user1ID.String()}, participantIDs(left))
	})

	t.Run("creator cannot leave", func(t *testing.T) {
		g := createGame(t)
		resp := doAuth(t, http.MethodPost, fmt.Sprintf("/api/games/%d/leave", g.Game.ID), nil, token1)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
		assert.Equal(t, "CREATOR_CANNOT_LEAVE", errCode(t, resp))
	})

	t.Run("non-participant cannot leave", func(t *testing.T) {
		g := createGame(t)
		token3 := authToken(t, "outsider-leave@test.com")
		resp := doAuth(t, http.MethodPost, fmt.Sprintf("/api/games/%d/leave", g.Game.ID), nil, token3)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
		assert.Equal(t, "NOT_PARTICIPANT", errCode(t, resp))
	})

	t.Run("cannot leave active game", func(t *testing.T) {
		g := createActiveGame(t)
		resp := doAuth(t, http.MethodPost, fmt.Sprintf("/api/games/%d/leave", g.Game.ID), nil, token2)
		assert.Equal(t, http.StatusConflict, resp.StatusCode)
		assert.Equal(t, "GAME_ALREADY_STARTED", errCode(t, resp))
	})

	t.Run("cannot leave cancelled game", func(t *testing.T) {
		g := createGame(t)
		resp := doAuth(t, http.MethodPost, fmt.Sprintf("/api/games/%d/cancel", g.Game.ID), nil, token1)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()

		resp = doAuth(t, http.MethodPost, fmt.Sprintf("/api/games/%d/leave", g.Game.ID), nil, token2)
		assert.Equal(t, http.StatusConflict, resp.StatusCode)
		assert.Equal(t, "GAME_ALREADY_CANCELLED", errCode(t, resp))
	})

	t.Run("non-existent game", func(t *testing.T) {
		resp := doAuth(t, http.MethodPost, "/api/games/999999/leave", nil, token1)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
		assert.Equal(t, "GAME_NOT_FOUND", errCode(t, resp))
	})
}
