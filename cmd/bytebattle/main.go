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
	defer db.Close()

	userRepo := database.NewUserRepository(db)
	userService := service.NewUserService(userRepo)

	duelRepo := database.NewDuelRepository(db)
	duelService := service.NewDuelService(duelRepo)

	srv := server.NewHTTPServer(userService, duelService)

	log.Printf("Server started on %s", httpCfg.Address)
	log.Fatal(srv.Run(httpCfg.Address()))
}
