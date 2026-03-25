package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"bytebattle/internal/app"
	"bytebattle/internal/config"
	"bytebattle/internal/migrations"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	cfg := config.Load()

	pool, err := pgxpool.New(context.Background(), cfg.DBDSN)
	if err != nil {
		log.Fatalf("db pool error: %v", err)
	}

	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		log.Fatalf("db ping error: %v", err)
	}

	if len(os.Args) > 1 && os.Args[1] == "migrate" {
		if err := migrations.Run(pool); err != nil {
			log.Fatalf("migration error: %v", err)
		}
		log.Println("Migrations applied successfully")
		return
	}

	defer pool.Close()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           app.NewRouter(pool, cfg),
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
