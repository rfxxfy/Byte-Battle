package app

import (
	"log"
	"net/http"

	sqlcdb "bytebattle/internal/db/sqlc"
	"bytebattle/internal/executor"
	"bytebattle/internal/server"
	"bytebattle/internal/service"
	"bytebattle/internal/ws"

	"github.com/jackc/pgx/v5/pgxpool"
)

func NewRouter(pool *pgxpool.Pool) http.Handler {
	execCfg := executor.DefaultConfig()
	if cfg, err := executor.LoadConfig("executor_config.json"); err == nil {
		execCfg = cfg
	}

	dockerExecutor, err := executor.NewDockerExecutor(execCfg)
	if err != nil {
		log.Fatalf("failed to create executor: %v", err)
	}

	return NewRouterWithExecutor(pool, dockerExecutor)
}

func NewRouterWithExecutor(pool *pgxpool.Pool, exec executor.Executor) http.Handler {
	q := sqlcdb.New(pool)

	userService := service.NewUserService(q)
	gameService := service.NewGameService(q, pool)
	sessionService := service.NewSessionService(q)
	executionService := service.NewExecutionService(exec)

	hub := ws.NewHub()
	return server.New(userService, gameService, sessionService, executionService, hub)
}
