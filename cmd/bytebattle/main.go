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
	authCfg := config.LoadAuthConfig()
	smtpCfg := config.LoadSMTPConfig()

	db, err := database.NewPostgres(dbCfg)
	if err != nil {
		log.Fatalf("db error: %v", err)
	}
	defer db.Close()

	userRepo := database.NewUserRepository(db)
	duelRepo := database.NewDuelRepository(db)
	verificationRepo := database.NewVerificationRepository(db)
	sessionRepo := database.NewSessionRepository(db)

	userService := service.NewUserService(userRepo)
	duelService := service.NewDuelService(duelRepo)

	mailer := service.NewMailer(smtpCfg)
	authService := service.NewAuthService(userRepo, verificationRepo, sessionRepo, mailer, authCfg)

	srv := server.NewHTTPServer(userService, duelService, authService, authCfg)

	addr := httpCfg.Address()
	log.Printf("Server started on %s", addr)
	log.Fatal(srv.Run(addr))
}
