package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	DBDSN       string
	HTTPAddr    string
	ProblemsDir string
	Entrance    EntranceConfig
}

type EntranceConfig struct {
	ResendAPIKey    string
	FromEmail       string
	CodeTTL         time.Duration
	MaxAttempts     int
	BcryptCost      int
	SessionTTL      time.Duration
	CookieName      string
	CookieSecure    bool
}

func Load() Config {
	return Config{
		DBDSN: getEnv("DB_DSN", "postgres://bytebattle:bytebattle@localhost:5432/bytebattle?sslmode=disable"),
		HTTPAddr: fmt.Sprintf("%s:%s",
			getEnv("HTTP_HOST", "0.0.0.0"),
			getEnv("HTTP_PORT", "8080"),
		),
		ProblemsDir: getEnv("PROBLEMS_DIR", "./problems"),
		Entrance: EntranceConfig{
			ResendAPIKey: getEnv("RESEND_API_KEY", ""),
			FromEmail:    getEnv("FROM_EMAIL", "noreply@bytebattle.dev"),
			CodeTTL:      getDurationEnv("ENTRANCE_CODE_TTL", 15*time.Minute),
			MaxAttempts:  getIntEnv("ENTRANCE_MAX_ATTEMPTS", 5),
			BcryptCost:   getIntEnv("ENTRANCE_BCRYPT_COST", 10),
			SessionTTL:   getDurationEnv("SESSION_TTL", 24*time.Hour),
			CookieName:   getEnv("SESSION_COOKIE_NAME", "bb_session"),
			CookieSecure: getEnv("COOKIE_SECURE", "false") == "true",
		},
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getIntEnv(key string, defaultValue int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return defaultValue
}

func getDurationEnv(key string, defaultValue time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return defaultValue
}
