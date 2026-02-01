package main

import (
	"log"

	"bytebattle/internal/config"
	"bytebattle/internal/database"
	"bytebattle/internal/executor"
	"bytebattle/internal/server"
	"bytebattle/internal/service"
)

func main() {
	dbCfg := config.LoadDatabaseConfig()
	httpCfg := config.LoadHTTPConfig()

	db, err := database.NewPostgres(dbCfg)
	if err != nil {
		log.Fatalf("db error: %v", err)
	}
	defer db.Close()

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

	gameRepo := database.NewDuelRepository(db)
	gameService := service.NewGameService(gameRepo)

	sessionRepo := database.NewSessionRepository(db)
	sessionService := service.NewSessionService(sessionRepo)

	srv := server.NewHTTPServer(userService, gameService, sessionService, executionService)

	addr := httpCfg.Address()
	log.Printf("Server started on %s", addr)
	log.Fatal(srv.Run(addr))
}
