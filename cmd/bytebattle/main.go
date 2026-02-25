package main

import (
	"fmt"
	"log"

	"bytebattle/internal/config"
	"bytebattle/internal/database"
	"bytebattle/internal/executor"
	"bytebattle/internal/server"
	"bytebattle/internal/service"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("%v", err)
	}
}

func run() error {
	dbCfg := config.LoadDatabaseConfig()
	httpCfg := config.LoadHTTPConfig()

	db, err := database.NewPostgres(dbCfg)
	if err != nil {
		return fmt.Errorf("db: %w", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("failed to close database: %v", err)
		}
	}()

	execCfg := executor.DefaultConfig()
	if cfg, err := executor.LoadConfig("executor_config.json"); err == nil {
		execCfg = cfg
	}

	dockerExecutor, err := executor.NewDockerExecutor(execCfg)
	if err != nil {
		return fmt.Errorf("create executor: %w", err)
	}
	executionService := service.NewExecutionService(dockerExecutor)

	userRepo := database.NewUserRepository(db)
	userService := service.NewUserService(userRepo)

	gameRepo := database.NewGameRepository(db)
	gameService := service.NewGameService(gameRepo)

	sessionRepo := database.NewSessionRepository(db)
	sessionService := service.NewSessionService(sessionRepo)

	srv := server.NewHTTPServer(userService, gameService, sessionService, executionService)

	addr := httpCfg.Address()
	log.Printf("Server started on %s", addr)
	if err := srv.Run(addr); err != nil {
		return fmt.Errorf("server: %w", err)
	}
	return nil
}
