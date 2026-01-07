package config

import (
	"fmt"
	"os"
)

type Config struct {
	DBDSN    string
	HTTPAddr string
}

func Load() Config {
	return Config{
		DBDSN: getEnv("DB_DSN", "postgres://bytebattle:bytebattle@localhost:5432/bytebattle?sslmode=disable"),
		HTTPAddr: fmt.Sprintf("%s:%s",
			getEnv("HTTP_HOST", "0.0.0.0"),
			getEnv("HTTP_PORT", "8080"),
		),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
