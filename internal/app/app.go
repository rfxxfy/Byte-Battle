package app

import (
	"log"
	"net/http"

	sqlcdb "bytebattle/internal/db/sqlc"
	"bytebattle/internal/executor"
	"bytebattle/internal/problems"
	"bytebattle/internal/server"
	"bytebattle/internal/service"
	"bytebattle/internal/ws"

	"github.com/jackc/pgx/v5/pgxpool"
)

func NewRouter(pool *pgxpool.Pool, problemsDir string) http.Handler {
	execCfg := executor.DefaultConfig()
	if cfg, err := executor.LoadConfig("executor_config.json"); err == nil {
		execCfg = cfg
	}

	dockerExecutor, err := executor.NewDockerExecutor(execCfg)
	if err != nil {
		log.Fatalf("failed to create executor: %v", err)
	}

	loader, err := problems.NewLoader(problemsDir)
	if err != nil {
		log.Fatalf("failed to load problems: %v", err)
	}

	return NewRouterWithExecutor(pool, dockerExecutor, loader)
}

func NewRouterWithExecutor(pool *pgxpool.Pool, exec executor.Executor, loader *problems.Loader) http.Handler {
	q := sqlcdb.New(pool)

	userService := service.NewUserService(q)
	gameService := service.NewGameService(q, pool, loader)
	problemService := service.NewProblemService(loader)
	sessionService := service.NewSessionService(q)
	executionService := service.NewExecutionService(exec)

	hub := ws.NewHub()
	return server.New(userService, gameService, problemService, sessionService, executionService, hub)
}
