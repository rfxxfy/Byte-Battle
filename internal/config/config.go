package config

import (
	"log"
	"os"
	"strconv"
)

// DatabaseConfig - структура для хранения настроек подключения к базе данных
type DatabaseConfig struct {
	Host     string // Хост базы данных
	Port     int    // Порт базы данных
	User     string // Имя пользователя
	Password string // Пароль
	Name     string // Имя базы данных
	SSLMode  string // Режим SSL
}

// LoadDatabaseConfig - загружает настройки базы данных из переменных окружения
func LoadDatabaseConfig() *DatabaseConfig {
	port, _ := strconv.Atoi(getEnv("DB_PORT", "5432"))

	return &DatabaseConfig{
		Host:     requireEnv("DB_HOST"),
		Port:     port,
		User:     requireEnv("DB_USER"),
		Password: requireEnv("DB_PASSWORD"),
		Name:     requireEnv("DB_NAME"),
		SSLMode:  requireEnv("DB_SSLMODE"),
	}
}

// getEnv - возвращает значение переменной окружения или значение по умолчанию
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// requireEnv - возвращает значение переменной окружения или завершает процесс с ошибкой
func requireEnv(key string) string {
	value := os.Getenv(key)
	if value == "" {
		log.Fatalf("required environment variable %s is not set", key)
	}
	return value
}
