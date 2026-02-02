package e2e_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"bytebattle/internal/app"
	"bytebattle/internal/config"
	"bytebattle/internal/ws"
)

func TestGameWS_InvalidParams(t *testing.T) {
	t.Run("missing token", func(t *testing.T) {
		g := createActiveGame(t)
		_, resp := wsDial(t, fmt.Sprintf("/games/%d/ws", g.Game.ID), "")
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("invalid token", func(t *testing.T) {
		g := createActiveGame(t)
		_, resp := wsDial(t, fmt.Sprintf("/games/%d/ws", g.Game.ID), "badtoken")
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("nonexistent game", func(t *testing.T) {
		_, resp := wsDial(t, "/games/999999/ws", token1)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}

func TestGameWS_PendingGame(t *testing.T) {
	g := createGame(t) // pending, not started
	_, resp := wsDial(t, fmt.Sprintf("/games/%d/ws", g.Game.ID), token1)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestGameWS_SubmitBroadcastsToAllClients(t *testing.T) {
	g := createActiveGame(t)
	wsPath := fmt.Sprintf("/games/%d/ws", g.Game.ID)

	conn1 := wsConnect(t, wsPath, token1)
	conn2 := wsConnect(t, wsPath, token2)

	require.NoError(t, conn1.WriteJSON(ws.ClientMessage{
		Type:     ws.TypeSubmit,
		Code:     "print('hello')",
		Language: "python",
	}))

	r1 := wsRead(t, conn1)
	assert.Equal(t, ws.TypeSubmissionResult, r1.Type)
	assert.Equal(t, user1ID, r1.UserID)
	assert.True(t, r1.Accepted)

	r2 := wsRead(t, conn2)
	assert.Equal(t, ws.TypeSubmissionResult, r2.Type)
	assert.Equal(t, user1ID, r2.UserID)
	assert.True(t, r2.Accepted)
}

func TestGameWS_RejectedSubmitDoesNotFinishGame(t *testing.T) {
	// failingExecutor always returns ExitCode=1, so no game_finished should be sent
	srv := httptest.NewServer(app.NewRouterWithExecutor(testPool, failingExecutor{}, testLoader, config.Load()))
	t.Cleanup(srv.Close)

	srvURL := srv.URL
	doFailing := func(method, path string, body any) *http.Response {
		var buf bytes.Buffer
		if body != nil {
			require.NoError(t, json.NewEncoder(&buf).Encode(body))
		}
		req, err := http.NewRequest(method, srvURL+path, &buf)
		require.NoError(t, err)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		req.Header.Set("Authorization", "Bearer "+token1)
		resp, err := srv.Client().Do(req)
		require.NoError(t, err)
		return resp
	}

	r := doFailing(http.MethodPost, "/games", map[string]any{
		"problem_id": "test-problem",
	})
	require.Equal(t, http.StatusCreated, r.StatusCode)
	var g gameResp
	decodeJSON(t, r, &g)

	// user2 joins via the main test server (same DB)
	joinResp := doAuth(t, http.MethodPost, fmt.Sprintf("/games/%d/join", g.Game.ID), nil, token2)
	require.Equal(t, http.StatusOK, joinResp.StatusCode)
	joinResp.Body.Close()

	r = doFailing(http.MethodPost, fmt.Sprintf("/games/%d/start", g.Game.ID), nil)
	require.Equal(t, http.StatusOK, r.StatusCode)
	r.Body.Close()

	wsURLFailing := "ws" + strings.TrimPrefix(srvURL, "http") + fmt.Sprintf("/games/%d/ws", g.Game.ID)
	conn, _, err := wsDialer(token1).Dial(wsURLFailing, nil)
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	require.NoError(t, conn.WriteJSON(ws.ClientMessage{
		Type:     ws.TypeSubmit,
		Code:     "wrong solution",
		Language: "go",
	}))

	conn.SetReadDeadline(time.Now().Add(3 * time.Second)) //nolint:errcheck // test helper, error not actionable
	var result ws.ServerMessage
	require.NoError(t, conn.ReadJSON(&result))
	assert.Equal(t, ws.TypeSubmissionResult, result.Type)
	assert.False(t, result.Accepted)

	// no game_finished should arrive
	conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond)) //nolint:errcheck // test helper, error not actionable
	var unexpected ws.ServerMessage
	err = conn.ReadJSON(&unexpected)
	assert.Error(t, err, "expected timeout, not a game_finished message")
}

func TestGameWS_AcceptedSubmitFinishesGame(t *testing.T) {
	g := createActiveGame(t)
	conn := wsConnect(t, fmt.Sprintf("/games/%d/ws", g.Game.ID), token1)

	require.NoError(t, conn.WriteJSON(ws.ClientMessage{
		Type:     ws.TypeSubmit,
		Code:     "solution",
		Language: "go",
	}))

	result := wsRead(t, conn)
	assert.Equal(t, ws.TypeSubmissionResult, result.Type)
	assert.True(t, result.Accepted)

	finished := wsRead(t, conn)
	assert.Equal(t, ws.TypeGameFinished, finished.Type)
	assert.Equal(t, user1ID, finished.WinnerID)
}
