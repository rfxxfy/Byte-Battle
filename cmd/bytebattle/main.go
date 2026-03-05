package main

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"bytebattle/internal/app"
	"bytebattle/internal/config"
	"bytebattle/internal/database"
)

func main() {
	dbCfg := config.LoadDatabaseConfig()
	httpCfg := config.LoadHTTPConfig()

	db, err := database.NewPostgres(dbCfg)
	if err != nil {
		log.Fatalf("db error: %v", err)
	}
	defer db.Close()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	srv := &http.Server{
		Addr:              httpCfg.Address(),
		Handler:           app.NewRouter(db),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("Server started on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-ctx.Done()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
	log.Printf("Server shut down")
}
