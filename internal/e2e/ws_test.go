package e2e_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"

	"bytebattle/internal/app"
	"bytebattle/internal/config"
	"bytebattle/internal/service"
	"bytebattle/internal/ws"
)

func TestGameWS_InvalidParams(t *testing.T) {
	t.Run("missing token", func(t *testing.T) {
		g := createActiveGame(t)
		_, resp := wsDial(t, fmt.Sprintf("/api/games/%d/ws", g.Game.ID), "")
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("invalid token", func(t *testing.T) {
		g := createActiveGame(t)
		_, resp := wsDial(t, fmt.Sprintf("/api/games/%d/ws", g.Game.ID), "badtoken")
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("nonexistent game", func(t *testing.T) {
		_, resp := wsDial(t, "/api/games/999999/ws", token1)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}

func TestGameWS_PendingGame(t *testing.T) {
	g := createGame(t) // pending, not started
	_, resp := wsDial(t, fmt.Sprintf("/api/games/%d/ws", g.Game.ID), token1)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestGameWS_SubmitBroadcastsToAllClients(t *testing.T) {
	g := createActiveGame(t)
	wsPath := fmt.Sprintf("/api/games/%d/ws", g.Game.ID)

	conn1 := wsConnect(t, wsPath, token1)
	joined1 := wsReadUntilType(t, conn1, ws.TypePlayerJoined)
	assert.Equal(t, user1ID, joined1.UserID)
	conn2 := wsConnect(t, wsPath, token2)
	joined2ForConn1 := wsReadUntilType(t, conn1, ws.TypePlayerJoined)
	assert.Equal(t, user2ID, joined2ForConn1.UserID)
	joined2ForConn2 := wsReadUntilType(t, conn2, ws.TypePlayerJoined)
	assert.Equal(t, user2ID, joined2ForConn2.UserID)

	require.NoError(t, conn1.WriteJSON(ws.ClientMessage{
		Type:     ws.TypeSubmit,
		Code:     "print('hello')",
		Language: "python",
	}))

	r1 := wsReadUntilType(t, conn1, ws.TypeSubmissionResult)
	assert.Equal(t, ws.TypeSubmissionResult, r1.Type)
	assert.Equal(t, user1ID, r1.UserID)
	assert.True(t, r1.Accepted)

	r2 := wsReadUntilType(t, conn2, ws.TypeSubmissionResult)
	assert.Equal(t, ws.TypeSubmissionResult, r2.Type)
	assert.Equal(t, user1ID, r2.UserID)
	assert.True(t, r2.Accepted)
}

func TestGameWS_PlayerJoinedBroadcast(t *testing.T) {
	g := createActiveGame(t)
	wsPath := fmt.Sprintf("/api/games/%d/ws", g.Game.ID)

	conn1 := wsConnect(t, wsPath, token1)
	joined1 := wsReadUntilType(t, conn1, ws.TypePlayerJoined)
	assert.Equal(t, user1ID, joined1.UserID)

	conn2 := wsConnect(t, wsPath, token2)
	joined2ForConn1 := wsReadUntilType(t, conn1, ws.TypePlayerJoined)
	assert.Equal(t, user2ID, joined2ForConn1.UserID)
	joined2ForConn2 := wsReadUntilType(t, conn2, ws.TypePlayerJoined)
	assert.Equal(t, user2ID, joined2ForConn2.UserID)
}

func TestGameWS_RejectedSubmitDoesNotFinishGame(t *testing.T) {
	// failingExecutor returns wrong stdout, so no game_finished should be sent
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

	r := doFailing(http.MethodPost, "/api/games", map[string]any{
		"problem_ids": []string{"test-problem"},
	})
	require.Equal(t, http.StatusCreated, r.StatusCode)
	var g gameResp
	decodeJSON(t, r, &g)

	// user2 joins
	joinResp := doAuth(t, http.MethodPost, fmt.Sprintf("/api/games/%d/join", g.Game.ID), nil, token2)
	require.Equal(t, http.StatusOK, joinResp.StatusCode)
	joinResp.Body.Close()

	r = doFailing(http.MethodPost, fmt.Sprintf("/api/games/%d/start", g.Game.ID), nil)
	require.Equal(t, http.StatusOK, r.StatusCode)
	r.Body.Close()

	wsURLFailing := "ws" + strings.TrimPrefix(srvURL, "http") + fmt.Sprintf("/api/games/%d/ws", g.Game.ID)
	conn, _, err := wsDialer(token1).Dial(wsURLFailing, nil)
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	require.NoError(t, conn.WriteJSON(ws.ClientMessage{
		Type:     ws.TypeSubmit,
		Code:     "wrong solution",
		Language: "go",
	}))

	result := wsReadUntilType(t, conn, ws.TypeSubmissionResult)
	assert.Equal(t, ws.TypeSubmissionResult, result.Type)
	assert.False(t, result.Accepted)
	if assert.NotNil(t, result.FailedTest) {
		assert.Equal(t, 0, *result.FailedTest)
	}

	// no game_finished should arrive
	conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond)) //nolint:errcheck // test helper, error not actionable
	var unexpected ws.ServerMessage
	err = conn.ReadJSON(&unexpected)
	assert.Error(t, err, "expected timeout, not a game_finished message")
}

func TestGameWS_FailedTestIndexIsCorrect(t *testing.T) {
	// secondTestFailsExecutor passes test 0 but fails test 1, failed_test must be 1
	srv := httptest.NewServer(app.NewRouterWithExecutor(testPool, &secondTestFailsExecutor{}, testLoader, config.Load()))
	t.Cleanup(srv.Close)

	srvURL := srv.URL
	doSrv := func(method, path string, body any, token string) *http.Response {
		var buf bytes.Buffer
		if body != nil {
			require.NoError(t, json.NewEncoder(&buf).Encode(body))
		}
		req, err := http.NewRequest(method, srvURL+path, &buf)
		require.NoError(t, err)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := srv.Client().Do(req)
		require.NoError(t, err)
		return resp
	}

	r := doSrv(http.MethodPost, "/api/games", map[string]any{"problem_ids": []string{"test-problem"}}, token1)
	require.Equal(t, http.StatusCreated, r.StatusCode)
	var g gameResp
	decodeJSON(t, r, &g)

	joinResp := doAuth(t, http.MethodPost, fmt.Sprintf("/api/games/%d/join", g.Game.ID), nil, token2)
	require.Equal(t, http.StatusOK, joinResp.StatusCode)
	joinResp.Body.Close()

	r = doSrv(http.MethodPost, fmt.Sprintf("/api/games/%d/start", g.Game.ID), nil, token1)
	require.Equal(t, http.StatusOK, r.StatusCode)
	r.Body.Close()

	wsURLSrv := "ws" + strings.TrimPrefix(srvURL, "http") + fmt.Sprintf("/api/games/%d/ws", g.Game.ID)
	conn, _, err := wsDialer(token1).Dial(wsURLSrv, nil)
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	require.NoError(t, conn.WriteJSON(ws.ClientMessage{
		Type:     ws.TypeSubmit,
		Code:     "partial solution",
		Language: "go",
	}))

	result := wsReadUntilType(t, conn, ws.TypeSubmissionResult)
	assert.Equal(t, ws.TypeSubmissionResult, result.Type)
	assert.False(t, result.Accepted)
	if assert.NotNil(t, result.FailedTest) {
		assert.Equal(t, 1, *result.FailedTest)
	}
}

func TestGameWS_AcceptedSubmitFinishesGame(t *testing.T) {
	g := createActiveGame(t)
	conn := wsConnect(t, fmt.Sprintf("/api/games/%d/ws", g.Game.ID), token1)
	joined := wsReadUntilType(t, conn, ws.TypePlayerJoined)
	assert.Equal(t, user1ID, joined.UserID)

	require.NoError(t, conn.WriteJSON(ws.ClientMessage{
		Type:     ws.TypeSubmit,
		Code:     "solution",
		Language: "go",
	}))

	result := wsReadUntilType(t, conn, ws.TypeSubmissionResult)
	assert.Equal(t, ws.TypeSubmissionResult, result.Type)
	assert.True(t, result.Accepted)

	finished := wsReadUntilType(t, conn, ws.TypeGameFinished)
	assert.Equal(t, ws.TypeGameFinished, finished.Type)
	assert.Equal(t, user1ID, finished.WinnerID)
}

func TestGameWS_AcceptedSubmitAdvancesRound(t *testing.T) {
	// Use isolated server with unlimited rate to avoid shared token limiter flakiness.
	srv := newGameServer(t, correctExecutor{}, service.RateLimitConfig{Rate: rate.Inf, Burst: 100})

	resp := doOnServer(t, srv, http.MethodPost, "/api/games", map[string]any{
		"problem_ids": []string{"test-problem", "test-problem"},
	}, token1)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var g gameResp
	decodeJSON(t, resp, &g)

	joinResp := doOnServer(t, srv, http.MethodPost, fmt.Sprintf("/api/games/%d/join", g.Game.ID), nil, token2)
	require.Equal(t, http.StatusOK, joinResp.StatusCode)
	joinResp.Body.Close()

	startResp := doOnServer(t, srv, http.MethodPost, fmt.Sprintf("/api/games/%d/start", g.Game.ID), nil, token1)
	require.Equal(t, http.StatusOK, startResp.StatusCode)
	startResp.Body.Close()

	conn := wsConnectOnServer(t, srv, fmt.Sprintf("/api/games/%d/ws", g.Game.ID), token1)
	wsReadUntilType(t, conn, ws.TypePlayerJoined)

	require.NoError(t, conn.WriteJSON(ws.ClientMessage{
		Type:     ws.TypeSubmit,
		Code:     "solution",
		Language: "go",
	}))

	result := wsReadUntilType(t, conn, ws.TypeSubmissionResult)
	assert.True(t, result.Accepted)

	round := wsReadUntilType(t, conn, ws.TypeRoundAdvanced)
	assert.Equal(t, "test-problem", round.ProblemID)
	assert.Equal(t, 1, round.ProblemIdx)

	require.NoError(t, conn.WriteJSON(ws.ClientMessage{
		Type:     ws.TypeSubmit,
		Code:     "solution",
		Language: "go",
	}))

	secondResult := wsReadUntilType(t, conn, ws.TypeSubmissionResult)
	assert.True(t, secondResult.Accepted)

	finished := wsReadUntilType(t, conn, ws.TypeGameFinished)
	assert.Equal(t, user1ID, finished.WinnerID)
}

func TestGameWS_ConcurrentSlotRejected(t *testing.T) {
	exec := newBlockingExecutor()
	srv := newGameServer(t, exec)
	t.Cleanup(func() { close(exec.unblock) }) // unblock so goroutines can exit

	g := createActiveGameOnServer(t, srv)
	conn := wsConnectOnServer(t, srv, fmt.Sprintf("/api/games/%d/ws", g.Game.ID), token1)
	wsReadUntilType(t, conn, ws.TypePlayerJoined)

	// first submit — will block inside the executor, holding the slot
	require.NoError(t, conn.WriteJSON(ws.ClientMessage{Type: ws.TypeSubmit, Code: "x", Language: "go"}))

	// wait until the executor has actually started (slot is held)
	<-exec.started

	// second submit while slot is held — should get an error broadcast
	require.NoError(t, conn.WriteJSON(ws.ClientMessage{Type: ws.TypeSubmit, Code: "x", Language: "go"}))
	errMsg := wsReadUntilType(t, conn, ws.TypeError)
	assert.Contains(t, errMsg.Message, "in progress")
}

func TestGameWS_TwoPlayersRace_OnlyOneWins(t *testing.T) {
	// Use a custom server with unlimited rate to avoid cross-test token exhaustion.
	srv := newGameServer(t, correctExecutor{}, service.RateLimitConfig{Rate: rate.Inf, Burst: 100})
	g := createActiveGameOnServer(t, srv)
	wsPath := fmt.Sprintf("/api/games/%d/ws", g.Game.ID)

	conn1 := wsConnectOnServer(t, srv, wsPath, token1)
	wsReadUntilType(t, conn1, ws.TypePlayerJoined)
	conn2 := wsConnectOnServer(t, srv, wsPath, token2)
	wsReadUntilType(t, conn1, ws.TypePlayerJoined) // user2 joined (on conn1)
	wsReadUntilType(t, conn2, ws.TypePlayerJoined)

	// both players submit simultaneously
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		conn1.WriteJSON(ws.ClientMessage{Type: ws.TypeSubmit, Code: "solution", Language: "go"}) //nolint:errcheck // test goroutine, failure caught by wsReadUntilType timeout
	}()
	go func() {
		defer wg.Done()
		conn2.WriteJSON(ws.ClientMessage{Type: ws.TypeSubmit, Code: "solution", Language: "go"}) //nolint:errcheck // test goroutine, failure caught by wsReadUntilType timeout
	}()
	wg.Wait()

	// both connections must see exactly one game_finished broadcast with the same winner
	finished1 := wsReadUntilType(t, conn1, ws.TypeGameFinished)
	finished2 := wsReadUntilType(t, conn2, ws.TypeGameFinished)

	assert.Equal(t, finished1.WinnerID, finished2.WinnerID, "both connections must see the same winner")
	assert.True(t, finished1.WinnerID == user1ID || finished1.WinnerID == user2ID, "winner must be one of the players")
}

func wsReadUntilType(t *testing.T, conn wsJSONReader, wantType string) ws.ServerMessage {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		require.NoError(t, conn.SetReadDeadline(deadline))
		var msg ws.ServerMessage
		require.NoError(t, conn.ReadJSON(&msg))
		if msg.Type == wantType {
			return msg
		}
	}
}

type wsJSONReader interface {
	ReadJSON(v any) error
	SetReadDeadline(t time.Time) error
}
