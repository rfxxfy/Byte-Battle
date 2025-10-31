package database

import (
	"database/sql"
	"fmt"

	"bytebattle/internal/config"

	"github.com/aarondl/sqlboiler/v4/boil"
	_ "github.com/lib/pq"
)

func NewPostgres(cfg *config.DatabaseConfig) (*sql.DB, error) {
	connStr := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host,
		cfg.Port,
		cfg.User,
		cfg.Password,
		cfg.Name,
		cfg.SSLMode,
	)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}

	boil.SetDB(db)

	return db, nil
}
