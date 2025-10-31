package database

import (
	"database/sql"
	"fmt"

	"bytebattle/internal/config"

	"github.com/aarondl/sqlboiler/v4/boil"
	_ "github.com/lib/pq"
)

// Client оборачивает соединение с базой данных и предоставляет доступ к моделям
type Client struct {
	DB *sql.DB
}

// NewClient создает новый клиент базы данных
func NewClient(cfg *config.DatabaseConfig) (*Client, error) {
	// Создаем строку подключения к базе данных
	connStr := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Name, cfg.SSLMode,
	)

	// Открываем соединение с базой данных
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("не удалось открыть базу данных: %w", err)
	}

	// Проверяем соединение
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("не удалось подключиться к базе данных: %w", err)
	}

	// Устанавливаем соединение с базой данных по умолчанию для SQLBoiler
	boil.SetDB(db)

	return &Client{
		DB: db,
	}, nil
}

// Close закрывает соединение с базой данных
func (c *Client) Close() error {
	return c.DB.Close()
}
