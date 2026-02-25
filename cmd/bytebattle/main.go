package main

import (
	"log"

	"bytebattle/internal/config"
	"bytebattle/internal/database"
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

	userRepo := database.NewUserRepository(db)
	userService := service.NewUserService(userRepo)

	gameRepo := database.NewGameRepository(db)
	gameService := service.NewGameService(gameRepo)

	sessionRepo := database.NewSessionRepository(db)
	sessionService := service.NewSessionService(sessionRepo)

	srv := server.NewHTTPServer(userService, gameService, sessionService)

	addr := httpCfg.Address()
	log.Printf("Server started on %s", addr)
	if err := srv.Run(addr); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
