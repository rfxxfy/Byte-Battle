package e2e_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"golang.org/x/crypto/bcrypt"

	"bytebattle/internal/app"
	"bytebattle/internal/config"
	sqlcdb "bytebattle/internal/db/sqlc"
	"bytebattle/internal/executor"
	"bytebattle/internal/migrations"
	"bytebattle/internal/problems"
	"bytebattle/internal/service"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// correctExecutor returns the expected output for the test problem (01.out = "3").
type correctExecutor struct{}

func (correctExecutor) Run(_ context.Context, _ executor.ExecutionRequest) (executor.ExecutionResult, error) {
	return executor.ExecutionResult{Stdout: "3"}, nil
}

// failingExecutor returns output that does not match any test case expected output.
type failingExecutor struct{}

func (failingExecutor) Run(_ context.Context, _ executor.ExecutionRequest) (executor.ExecutionResult, error) {
	return executor.ExecutionResult{Stdout: "wrong", Stderr: "wrong answer"}, nil
}

type secondTestFailsExecutor struct {
	mu    sync.Mutex
	calls int
}

func (e *secondTestFailsExecutor) Run(_ context.Context, _ executor.ExecutionRequest) (executor.ExecutionResult, error) {
	e.mu.Lock()
	n := e.calls
	e.calls++
	e.mu.Unlock()
	if n == 0 {
		return executor.ExecutionResult{Stdout: "3"}, nil
	}
	return executor.ExecutionResult{Stdout: "wrong"}, nil
}

var (
	testSrv    *httptest.Server
	testPool   *pgxpool.Pool
	testLoader *problems.Loader
	user1ID    uuid.UUID
	user2ID    uuid.UUID
	token1     string
	token2     string
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
		PasswordHash: pgtype.Text{String: "hash", Valid: true},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "create user1: %v\n", err)
		pool.Close()
		_ = pgCtr.Terminate(ctx)
		os.Exit(1)
	}
	user1ID = u1.ID

	u2, err := q.CreateUser(ctx, sqlcdb.CreateUserParams{
		Username:     "player2",
		Email:        "player2@test.com",
		PasswordHash: pgtype.Text{String: "hash", Valid: true},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "create user2: %v\n", err)
		pool.Close()
		_ = pgCtr.Terminate(ctx)
		os.Exit(1)
	}
	user2ID = u2.ID

	loader, err := problems.NewLoader("testdata/problems")
	if err != nil {
		fmt.Fprintf(os.Stderr, "load test problems: %v\n", err)
		pool.Close()
		_ = pgCtr.Terminate(ctx)
		os.Exit(1)
	}

	testPool = pool
	testLoader = loader
	testSrv = httptest.NewServer(app.NewRouterWithExecutor(pool, correctExecutor{}, loader, config.Load()))

	var tokErr error
	token1, tokErr = makeAuthToken("player1@test.com")
	if tokErr != nil {
		fmt.Fprintf(os.Stderr, "init token1: %v\n", tokErr)
		testSrv.Close()
		pool.Close()
		_ = pgCtr.Terminate(ctx)
		os.Exit(1)
	}
	token2, tokErr = makeAuthToken("player2@test.com")
	if tokErr != nil {
		fmt.Fprintf(os.Stderr, "init token2: %v\n", tokErr)
		testSrv.Close()
		pool.Close()
		_ = pgCtr.Terminate(ctx)
		os.Exit(1)
	}

	code := m.Run()

	testSrv.Close()
	pool.Close()
	_ = pgCtr.Terminate(ctx)
	os.Exit(code)
}

// makeAuthToken bypasses the email mailer by inserting a known verification code
// directly into the DB, then calls /auth/confirm to get a session token.
func makeAuthToken(email string) (string, error) {
	ctx := context.Background()
	q := sqlcdb.New(testPool)

	const code = "000001"
	hash, err := bcrypt.GenerateFromPassword([]byte(code), 4) // low cost for tests
	if err != nil {
		return "", fmt.Errorf("hash code: %w", err)
	}

	if _, err := q.UpsertVerificationCode(ctx, sqlcdb.UpsertVerificationCodeParams{
		Email:     email,
		CodeHash:  string(hash),
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(5 * time.Minute), Valid: true},
	}); err != nil {
		return "", fmt.Errorf("upsert code: %w", err)
	}

	body, _ := json.Marshal(map[string]string{"email": email, "code": code})
	resp, err := testSrv.Client().Post(testSrv.URL+"/api/auth/confirm", "application/json", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("post /auth/confirm: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("/auth/confirm returned %d", resp.StatusCode)
	}

	var result struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil || result.Token == "" {
		return "", fmt.Errorf("decode token: %w", err)
	}
	return result.Token, nil
}

func authToken(t *testing.T, email string) string {
	t.Helper()
	tok, err := makeAuthToken(email)
	require.NoError(t, err)
	return tok
}

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

func doAuth(t *testing.T, method, path string, body any, token string) *http.Response {
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
	req.Header.Set("Authorization", "Bearer "+token)
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

func wsURL(path string) string {
	return "ws" + testSrv.URL[len("http"):] + path
}

type blockingExecutor struct {
	once    sync.Once
	started chan struct{}
	unblock chan struct{}
}

func newBlockingExecutor() *blockingExecutor {
	return &blockingExecutor{
		started: make(chan struct{}),
		unblock: make(chan struct{}),
	}
}

func (e *blockingExecutor) Run(ctx context.Context, _ executor.ExecutionRequest) (executor.ExecutionResult, error) {
	e.once.Do(func() { close(e.started) })
	select {
	case <-e.unblock:
		return executor.ExecutionResult{Stdout: "wrong"}, nil
	case <-ctx.Done():
		return executor.ExecutionResult{}, ctx.Err()
	}
}

// newGameServer creates a test HTTP server with the given executor and optional rate limit config.
func newGameServer(t *testing.T, exec executor.Executor, rlCfg ...service.RateLimitConfig) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(app.NewRouterWithExecutor(testPool, exec, testLoader, config.Load(), rlCfg...))
	t.Cleanup(srv.Close)
	return srv
}

// doOnServer performs an authenticated HTTP request against a custom test server.
func doOnServer(t *testing.T, srv *httptest.Server, method, path string, body any, token string) *http.Response {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&buf).Encode(body))
	}
	req, err := http.NewRequest(method, srv.URL+path, &buf)
	require.NoError(t, err)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := srv.Client().Do(req)
	require.NoError(t, err)
	return resp
}

// createActiveGameOnServer creates a pending game as user1, has user2 join, then starts it.
func createActiveGameOnServer(t *testing.T, srv *httptest.Server) gameResp {
	t.Helper()
	r := doOnServer(t, srv, http.MethodPost, "/api/games", map[string]any{"problem_ids": []string{"test-problem"}}, token1)
	require.Equal(t, http.StatusCreated, r.StatusCode)
	var g gameResp
	decodeJSON(t, r, &g)

	r = doOnServer(t, srv, http.MethodPost, fmt.Sprintf("/api/games/%d/join", g.Game.ID), nil, token2)
	require.Equal(t, http.StatusOK, r.StatusCode)
	r.Body.Close()

	r = doOnServer(t, srv, http.MethodPost, fmt.Sprintf("/api/games/%d/start", g.Game.ID), nil, token1)
	require.Equal(t, http.StatusOK, r.StatusCode)
	var started gameResp
	decodeJSON(t, r, &started)
	return started
}

// wsConnectOnServer dials a WebSocket on a custom test server.
func wsConnectOnServer(t *testing.T, srv *httptest.Server, path, token string) *websocket.Conn {
	t.Helper()
	u := "ws" + srv.URL[len("http"):] + path
	conn, _, err := wsDialer(token).Dial(u, nil)
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })
	return conn
}
