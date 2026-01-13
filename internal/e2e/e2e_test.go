package e2e_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"bytebattle/internal/app"
	sqlcdb "bytebattle/internal/db/sqlc"
	"bytebattle/internal/executor"
	"bytebattle/internal/migrations"
	"bytebattle/internal/ws"

	"github.com/jackc/pgx/v5/pgxpool"
)

type noOpExecutor struct{}

func (noOpExecutor) Run(_ context.Context, _ executor.ExecutionRequest) (executor.ExecutionResult, error) {
	return executor.ExecutionResult{}, nil
}

type failingExecutor struct{}

func (failingExecutor) Run(_ context.Context, _ executor.ExecutionRequest) (executor.ExecutionResult, error) {
	return executor.ExecutionResult{ExitCode: 1, Stderr: "wrong answer"}, nil
}

var (
	testSrv   *httptest.Server
	testPool  *pgxpool.Pool
	user1ID   int
	user2ID   int
	problemID int
)

func TestMain(m *testing.M) {
	ctx := context.Background()

	pgCtr, err := tcpostgres.Run(ctx,
		"postgres:18-alpine",
		tcpostgres.WithDatabase("bytebattle"),
		tcpostgres.WithUsername("bytebattle"),
		tcpostgres.WithPassword("bytebattle"),
		tcpostgres.WithSQLDriver("pgx"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "start postgres container: %v\n", err)
		os.Exit(1)
	}

	dsn, err := pgCtr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		fmt.Fprintf(os.Stderr, "get connection string: %v\n", err)
		_ = pgCtr.Terminate(ctx)
		os.Exit(1)
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create pool: %v\n", err)
		_ = pgCtr.Terminate(ctx)
		os.Exit(1)
	}

	if err := migrations.Run(pool); err != nil {
		fmt.Fprintf(os.Stderr, "run migrations: %v\n", err)
		pool.Close()
		_ = pgCtr.Terminate(ctx)
		os.Exit(1)
	}

	q := sqlcdb.New(pool)

	u1, err := q.CreateUser(ctx, sqlcdb.CreateUserParams{
		Username:     "player1",
		Email:        "player1@test.com",
		PasswordHash: "hash",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "create user1: %v\n", err)
		pool.Close()
		_ = pgCtr.Terminate(ctx)
		os.Exit(1)
	}
	user1ID = int(u1.ID)

	u2, err := q.CreateUser(ctx, sqlcdb.CreateUserParams{
		Username:     "player2",
		Email:        "player2@test.com",
		PasswordHash: "hash",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "create user2: %v\n", err)
		pool.Close()
		_ = pgCtr.Terminate(ctx)
		os.Exit(1)
	}
	user2ID = int(u2.ID)

	var pid int32
	err = pool.QueryRow(ctx,
		"INSERT INTO problems (title, description, difficulty, time_limit, memory_limit) VALUES ($1, $2, $3, $4, $5) RETURNING id",
		"Test Problem", "A test problem", "easy", 5, 256,
	).Scan(&pid)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create problem: %v\n", err)
		pool.Close()
		_ = pgCtr.Terminate(ctx)
		os.Exit(1)
	}
	problemID = int(pid)

	testPool = pool
	testSrv = httptest.NewServer(app.NewRouterWithExecutor(pool, noOpExecutor{}))

	code := m.Run()

	testSrv.Close()
	pool.Close()
	_ = pgCtr.Terminate(ctx)
	os.Exit(code)
}

// --- helpers ---

func do(t *testing.T, method, path string, body any) *http.Response {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&buf).Encode(body))
	}
	req, err := http.NewRequest(method, testSrv.URL+path, &buf)
	require.NoError(t, err)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := testSrv.Client().Do(req)
	require.NoError(t, err)
	return resp
}

func decodeJSON(t *testing.T, resp *http.Response, dst any) {
	t.Helper()
	defer resp.Body.Close()
	require.NoError(t, json.NewDecoder(resp.Body).Decode(dst))
}

func errCode(t *testing.T, resp *http.Response) string {
	t.Helper()
	var body map[string]any
	decodeJSON(t, resp, &body)
	code, _ := body["error_code"].(string)
	return code
}

type gameResp struct {
	Game struct {
		ID        int    `json:"id"`
		ProblemID int    `json:"problem_id"`
		Status    string `json:"status"`
		WinnerID  *int   `json:"winner_id"`
	} `json:"game"`
}

type gamesListResp struct {
	Games []struct {
		ID     int    `json:"id"`
		Status string `json:"status"`
	} `json:"games"`
	Total int64 `json:"total"`
}

type sessionResp struct {
	Session struct {
		ID        int    `json:"id"`
		UserID    int    `json:"user_id"`
		Token     string `json:"token"`
		ExpiresAt string `json:"expires_at"`
	} `json:"session"`
}

func createGame(t *testing.T) gameResp {
	t.Helper()
	resp := do(t, http.MethodPost, "/games", map[string]any{
		"player_ids": []int{user1ID, user2ID},
		"problem_id": problemID,
	})
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var g gameResp
	decodeJSON(t, resp, &g)
	return g
}

func createActiveGame(t *testing.T) gameResp {
	t.Helper()
	g := createGame(t)
	resp := do(t, http.MethodPost, fmt.Sprintf("/games/%d/start", g.Game.ID), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var started gameResp
	decodeJSON(t, resp, &started)
	return started
}

// --- game tests ---

func TestGame_CreateAndGet(t *testing.T) {
	g := createGame(t)
	assert.Equal(t, "pending", g.Game.Status)
	assert.Equal(t, problemID, g.Game.ProblemID)

	resp := do(t, http.MethodGet, fmt.Sprintf("/games/%d", g.Game.ID), nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	var fetched gameResp
	decodeJSON(t, resp, &fetched)
	assert.Equal(t, g.Game.ID, fetched.Game.ID)
}

func TestGame_CreateValidation(t *testing.T) {
	t.Run("too few players", func(t *testing.T) {
		resp := do(t, http.MethodPost, "/games", map[string]any{
			"player_ids": []int{user1ID},
			"problem_id": problemID,
		})
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
		assert.Equal(t, "NOT_ENOUGH_PLAYERS", errCode(t, resp))
	})

	t.Run("duplicate players", func(t *testing.T) {
		resp := do(t, http.MethodPost, "/games", map[string]any{
			"player_ids": []int{user1ID, user1ID},
			"problem_id": problemID,
		})
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
		assert.Equal(t, "DUPLICATE_PLAYERS", errCode(t, resp))
	})
}

func TestGame_NotFound(t *testing.T) {
	const nonexistent = 999999

	resp := do(t, http.MethodGet, fmt.Sprintf("/games/%d", nonexistent), nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	assert.Equal(t, "GAME_NOT_FOUND", errCode(t, resp))

	resp = do(t, http.MethodPost, fmt.Sprintf("/games/%d/start", nonexistent), nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	resp = do(t, http.MethodPost, fmt.Sprintf("/games/%d/cancel", nonexistent), nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	resp = do(t, http.MethodDelete, fmt.Sprintf("/games/%d", nonexistent), nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestGame_List(t *testing.T) {
	// создаём несколько игр
	for range 3 {
		createGame(t)
	}

	resp := do(t, http.MethodGet, "/games?limit=2&offset=0", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var list gamesListResp
	decodeJSON(t, resp, &list)
	assert.Len(t, list.Games, 2)
	assert.GreaterOrEqual(t, list.Total, int64(3))
}

func TestGame_Delete(t *testing.T) {
	g := createGame(t)

	resp := do(t, http.MethodDelete, fmt.Sprintf("/games/%d", g.Game.ID), nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	resp = do(t, http.MethodGet, fmt.Sprintf("/games/%d", g.Game.ID), nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()
}

func TestGame_FullLifecycle(t *testing.T) {
	g := createGame(t)
	assert.Equal(t, "pending", g.Game.Status)

	// start
	resp := do(t, http.MethodPost, fmt.Sprintf("/games/%d/start", g.Game.ID), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var started gameResp
	decodeJSON(t, resp, &started)
	assert.Equal(t, "active", started.Game.Status)

	// complete
	resp = do(t, http.MethodPost, fmt.Sprintf("/games/%d/complete", g.Game.ID), map[string]any{
		"winner_id": user1ID,
	})
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var completed gameResp
	decodeJSON(t, resp, &completed)
	assert.Equal(t, "finished", completed.Game.Status)
	require.NotNil(t, completed.Game.WinnerID)
	assert.Equal(t, user1ID, *completed.Game.WinnerID)
}

func TestGame_CancelPending(t *testing.T) {
	g := createGame(t)

	resp := do(t, http.MethodPost, fmt.Sprintf("/games/%d/cancel", g.Game.ID), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var cancelled gameResp
	decodeJSON(t, resp, &cancelled)
	assert.Equal(t, "cancelled", cancelled.Game.Status)
}

func TestGame_InvalidTransitions(t *testing.T) {
	t.Run("start already active", func(t *testing.T) {
		g := createGame(t)
		resp := do(t, http.MethodPost, fmt.Sprintf("/games/%d/start", g.Game.ID), nil)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()

		resp = do(t, http.MethodPost, fmt.Sprintf("/games/%d/start", g.Game.ID), nil)
		assert.Equal(t, http.StatusConflict, resp.StatusCode)
		assert.Equal(t, "GAME_ALREADY_STARTED", errCode(t, resp))
	})

	t.Run("complete pending game", func(t *testing.T) {
		g := createGame(t)
		resp := do(t, http.MethodPost, fmt.Sprintf("/games/%d/complete", g.Game.ID), map[string]any{
			"winner_id": user1ID,
		})
		assert.Equal(t, http.StatusConflict, resp.StatusCode)
		assert.Equal(t, "GAME_NOT_IN_PROGRESS", errCode(t, resp))
	})

	t.Run("cancel finished game", func(t *testing.T) {
		g := createGame(t)
		resp := do(t, http.MethodPost, fmt.Sprintf("/games/%d/start", g.Game.ID), nil)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()

		resp = do(t, http.MethodPost, fmt.Sprintf("/games/%d/complete", g.Game.ID), map[string]any{
			"winner_id": user1ID,
		})
		require.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()

		resp = do(t, http.MethodPost, fmt.Sprintf("/games/%d/cancel", g.Game.ID), nil)
		assert.Equal(t, http.StatusConflict, resp.StatusCode)
		assert.Equal(t, "CANNOT_CANCEL_FINISHED_GAME", errCode(t, resp))
	})

	t.Run("cancel already cancelled", func(t *testing.T) {
		g := createGame(t)
		resp := do(t, http.MethodPost, fmt.Sprintf("/games/%d/cancel", g.Game.ID), nil)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()

		resp = do(t, http.MethodPost, fmt.Sprintf("/games/%d/cancel", g.Game.ID), nil)
		assert.Equal(t, http.StatusConflict, resp.StatusCode)
		assert.Equal(t, "GAME_ALREADY_CANCELLED", errCode(t, resp))
	})

	t.Run("complete with non-participant winner", func(t *testing.T) {
		g := createGame(t)
		resp := do(t, http.MethodPost, fmt.Sprintf("/games/%d/start", g.Game.ID), nil)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()

		resp = do(t, http.MethodPost, fmt.Sprintf("/games/%d/complete", g.Game.ID), map[string]any{
			"winner_id": 999999,
		})
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
		assert.Equal(t, "INVALID_WINNER", errCode(t, resp))
	})
}

// --- session tests ---

func createSession(t *testing.T, userID int) sessionResp {
	t.Helper()
	resp := do(t, http.MethodPost, "/sessions", map[string]any{
		"user_id": userID,
	})
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var s sessionResp
	decodeJSON(t, resp, &s)
	return s
}

func TestSession_CreateAndGet(t *testing.T) {
	s := createSession(t, user1ID)
	assert.Equal(t, user1ID, s.Session.UserID)
	assert.NotEmpty(t, s.Session.Token)
	assert.NotZero(t, s.Session.ID)

	resp := do(t, http.MethodGet, fmt.Sprintf("/sessions/%d", s.Session.ID), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var fetched sessionResp
	decodeJSON(t, resp, &fetched)
	assert.Equal(t, s.Session.ID, fetched.Session.ID)
	assert.Equal(t, s.Session.Token, fetched.Session.Token)
}

func TestSession_ValidateToken(t *testing.T) {
	s := createSession(t, user1ID)

	resp := do(t, http.MethodGet, fmt.Sprintf("/sessions/validate?token=%s", s.Session.Token), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var body struct {
		Valid   bool        `json:"valid"`
		Session sessionResp `json:"session"`
	}
	decodeJSON(t, resp, &body)
	assert.True(t, body.Valid)
}

func TestSession_InvalidToken(t *testing.T) {
	t.Run("empty token", func(t *testing.T) {
		resp := do(t, http.MethodGet, "/sessions/validate?token=", nil)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		assert.Equal(t, "INVALID_TOKEN", errCode(t, resp))
	})

	t.Run("nonexistent token", func(t *testing.T) {
		resp := do(t, http.MethodGet, "/sessions/validate?token=doesnotexist", nil)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
		assert.Equal(t, "SESSION_NOT_FOUND", errCode(t, resp))
	})
}

func TestSession_Refresh(t *testing.T) {
	s := createSession(t, user1ID)

	resp := do(t, http.MethodPost, fmt.Sprintf("/sessions/%d/refresh", s.Session.ID), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var refreshed sessionResp
	decodeJSON(t, resp, &refreshed)
	assert.Equal(t, s.Session.ID, refreshed.Session.ID)
	// новый expires_at должен быть позже старого
	oldExpiry, _ := time.Parse(time.RFC3339, s.Session.ExpiresAt)
	newExpiry, _ := time.Parse(time.RFC3339, refreshed.Session.ExpiresAt)
	assert.True(t, newExpiry.After(oldExpiry))
}

func TestSession_End(t *testing.T) {
	s := createSession(t, user1ID)

	resp := do(t, http.MethodDelete, fmt.Sprintf("/sessions/%d", s.Session.ID), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	resp = do(t, http.MethodGet, fmt.Sprintf("/sessions/%d", s.Session.ID), nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()
}

func TestSession_GetUserSessions(t *testing.T) {
	// создаём 2 сессии для user2
	createSession(t, user2ID)
	createSession(t, user2ID)

	resp := do(t, http.MethodGet, fmt.Sprintf("/users/%d/sessions", user2ID), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var body struct {
		Sessions []sessionResp `json:"sessions"`
	}
	decodeJSON(t, resp, &body)
	assert.GreaterOrEqual(t, len(body.Sessions), 2)
}

// --- websocket helpers ---

func wsURL(path string) string {
	return "ws" + strings.TrimPrefix(testSrv.URL, "http") + path
}

func wsDialer(token string) *websocket.Dialer {
	d := *websocket.DefaultDialer
	if token != "" {
		d.Subprotocols = []string{token}
	}
	return &d
}

func wsConnect(t *testing.T, path, token string) *websocket.Conn {
	t.Helper()
	conn, _, err := wsDialer(token).Dial(wsURL(path), nil)
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })
	return conn
}

func wsDial(t *testing.T, path, token string) (*websocket.Conn, *http.Response) {
	t.Helper()
	conn, resp, _ := wsDialer(token).Dial(wsURL(path), nil)
	if conn != nil {
		t.Cleanup(func() { conn.Close() })
	}
	return conn, resp
}

func wsRead(t *testing.T, conn *websocket.Conn) ws.ServerMessage {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(3 * time.Second)) //nolint:errcheck // test helper
	var msg ws.ServerMessage
	require.NoError(t, conn.ReadJSON(&msg))
	return msg
}

func sessionToken(t *testing.T, userID int) string {
	t.Helper()
	return createSession(t, userID).Session.Token
}

// --- websocket tests ---

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
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	t.Run("nonexistent game", func(t *testing.T) {
		_, resp := wsDial(t, "/games/999999/ws", sessionToken(t, user1ID))
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}

func TestGameWS_PendingGame(t *testing.T) {
	g := createGame(t) // pending, not started
	_, resp := wsDial(t, fmt.Sprintf("/games/%d/ws", g.Game.ID), sessionToken(t, user1ID))
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestGameWS_SubmitBroadcastsToAllClients(t *testing.T) {
	g := createActiveGame(t)
	wsPath := fmt.Sprintf("/games/%d/ws", g.Game.ID)

	conn1 := wsConnect(t, wsPath, sessionToken(t, user1ID))
	conn2 := wsConnect(t, wsPath, sessionToken(t, user2ID))

	require.NoError(t, conn1.WriteJSON(ws.ClientMessage{
		Type:     ws.TypeSubmit,
		Code:     "print('hello')",
		Language: "python",
	}))

	// оба клиента должны получить submission_result
	r1 := wsRead(t, conn1)
	assert.Equal(t, ws.TypeSubmissionResult, r1.Type)
	assert.Equal(t, int32(user1ID), r1.UserID)
	assert.True(t, r1.Accepted)

	r2 := wsRead(t, conn2)
	assert.Equal(t, ws.TypeSubmissionResult, r2.Type)
	assert.Equal(t, int32(user1ID), r2.UserID)
	assert.True(t, r2.Accepted)
}

func TestGameWS_RejectedSubmitDoesNotFinishGame(t *testing.T) {
	// failingExecutor always returns ExitCode=1, so no game_finished should be sent
	srv := httptest.NewServer(app.NewRouterWithExecutor(testPool, failingExecutor{}))
	t.Cleanup(srv.Close)

	wsURLFailing := func(path string) string {
		return "ws" + strings.TrimPrefix(srv.URL, "http") + path
	}
	doFailing := func(method, path string, body any) *http.Response {
		var buf bytes.Buffer
		if body != nil {
			require.NoError(t, json.NewEncoder(&buf).Encode(body))
		}
		req, err := http.NewRequest(method, srv.URL+path, &buf)
		require.NoError(t, err)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, err := srv.Client().Do(req)
		require.NoError(t, err)
		return resp
	}

	// create and start a game via the failing server
	r := doFailing(http.MethodPost, "/games", map[string]any{
		"player_ids": []int{user1ID, user2ID},
		"problem_id": problemID,
	})
	require.Equal(t, http.StatusCreated, r.StatusCode)
	var g gameResp
	decodeJSON(t, r, &g)

	r = doFailing(http.MethodPost, fmt.Sprintf("/games/%d/start", g.Game.ID), nil)
	require.Equal(t, http.StatusOK, r.StatusCode)
	r.Body.Close()

	token := sessionToken(t, user1ID)
	conn, _, err := wsDialer(token).Dial(wsURLFailing(fmt.Sprintf("/games/%d/ws", g.Game.ID)), nil)
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
	conn := wsConnect(t, fmt.Sprintf("/games/%d/ws", g.Game.ID), sessionToken(t, user1ID))

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
	assert.Equal(t, int32(user1ID), finished.WinnerID)
}
