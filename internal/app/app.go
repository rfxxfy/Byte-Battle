package app

import (
	"database/sql"
	"log"
	"net/http"

	"bytebattle/internal/database"
	"bytebattle/internal/executor"
	"bytebattle/internal/server"
	"bytebattle/internal/service"
)

func NewRouter(db *sql.DB) http.Handler {
	execCfg := executor.DefaultConfig()
	if cfg, err := executor.LoadConfig("executor_config.json"); err == nil {
		execCfg = cfg
	}

	dockerExecutor, err := executor.NewDockerExecutor(execCfg)
	if err != nil {
		log.Fatalf("failed to create executor: %v", err)
	}
	executionService := service.NewExecutionService(dockerExecutor)

	userRepo := database.NewUserRepository(db)
	userService := service.NewUserService(userRepo)

	gameRepo := database.NewGameRepository(db)
	gameService := service.NewGameService(gameRepo)

	sessionRepo := database.NewSessionRepository(db)
	sessionService := service.NewSessionService(sessionRepo)

	return server.New(userService, gameService, sessionService, executionService)
}
