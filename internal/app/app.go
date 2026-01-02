package app

import (
	"database/sql"
	"net/http"

	"bytebattle/internal/database"
	"bytebattle/internal/server"
	"bytebattle/internal/service"
)

func NewRouter(db *sql.DB) http.Handler {
	userRepo := database.NewUserRepository(db)
	userService := service.NewUserService(userRepo)

	gameRepo := database.NewGameRepository(db)
	gameService := service.NewGameService(gameRepo)

	sessionRepo := database.NewSessionRepository(db)
	sessionService := service.NewSessionService(sessionRepo)

	return server.New(userService, gameService, sessionService)
}
