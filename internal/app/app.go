package app

import (
	"log"
	"net/http"

	"bytebattle/internal/config"
	sqlcdb "bytebattle/internal/db/sqlc"
	"bytebattle/internal/executor"
	"bytebattle/internal/problems"
	"bytebattle/internal/server"
	"bytebattle/internal/service"
	"bytebattle/internal/ws"

	"github.com/jackc/pgx/v5/pgxpool"
)

func NewRouter(pool *pgxpool.Pool, cfg config.Config) http.Handler {
	execCfg := executor.DefaultConfig()
	if c, err := executor.LoadConfig("executor_config.json"); err == nil {
		execCfg = c
	}

	dockerExecutor, err := executor.NewDockerExecutor(execCfg)
	if err != nil {
		log.Fatalf("failed to create executor: %v", err)
	}

	loader, err := problems.NewLoader(cfg.ProblemsDir)
	if err != nil {
		log.Fatalf("failed to load problems: %v", err)
	}

	return NewRouterWithExecutor(pool, dockerExecutor, loader, cfg)
}

func NewRouterWithExecutor(pool *pgxpool.Pool, exec executor.Executor, loader *problems.Loader, cfg config.Config) http.Handler {
	q := sqlcdb.New(pool)

	userService := service.NewUserService(q)
	gameService := service.NewGameService(q, pool, loader)
	problemService := service.NewProblemService(loader)
	sessionService := service.NewSessionService(q, service.WithSessionDuration(cfg.Entrance.SessionTTL))
	executionService := service.NewExecutionService(exec)

	mailer := service.NewMailer(cfg.Entrance.ResendAPIKey, cfg.Entrance.FromEmail)
	entranceService := service.NewEntranceService(q, sessionService, mailer, cfg.Entrance)

	hub := ws.NewHub()
	return server.New(pool, userService, gameService, problemService, sessionService, executionService, hub, entranceService, cfg)
}
