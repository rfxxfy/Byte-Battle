package app

import (
	"context"
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
		if c.DockerHost != "" {
			execCfg.DockerHost = c.DockerHost
		}
		if len(c.Languages) > 0 {
			execCfg.Languages = c.Languages
		}
	}

	dockerExecutor, err := executor.NewDockerExecutor(execCfg)
	if err != nil {
		log.Fatalf("failed to create executor: %v", err)
	}

	store := problems.NewStore(cfg.ProblemsDir)

	if err := problems.SeedBuiltins(context.Background(), pool, store); err != nil {
		log.Fatalf("failed to seed built-in problems: %v", err)
	}

	return NewRouterWithExecutor(pool, dockerExecutor, store, cfg)
}

func NewRouterWithExecutor(pool *pgxpool.Pool, exec executor.Executor, store *problems.Store, cfg config.Config, rlCfg ...service.RateLimitConfig) http.Handler {
	q := sqlcdb.New(pool)

	userService := service.NewUserService(q)
	gameService := service.NewGameService(q, pool)
	problemService := service.NewProblemService(store, q)
	sessionService := service.NewSessionService(q, service.WithSessionDuration(cfg.Entrance.SessionTTL))
	executionService := service.NewExecutionService(exec, rlCfg...)
	submissionService := service.NewSubmissionService(executionService, gameService, store, q)

	mailer := service.NewMailer(cfg.Entrance.ResendAPIKey, cfg.Entrance.FromEmail)
	entranceService := service.NewEntranceService(q, sessionService, mailer, cfg.Entrance)

	hub := ws.NewHub()
	return server.New(pool, userService, gameService, problemService, sessionService, executionService, submissionService, hub, entranceService)
}
