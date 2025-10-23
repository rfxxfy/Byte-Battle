package config

import (
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
		Host:     getEnv("DB_HOST", "localhost"),
		Port:     port,
		User:     getEnv("DB_USER", "bytebattle"),
		Password: getEnv("DB_PASSWORD", "bytebattle"),
		Name:     getEnv("DB_NAME", "bytebattle"),
		SSLMode:  getEnv("DB_SSLMODE", "disable"),
	}
}

// getEnv - возвращает значение переменной окружения или значение по умолчанию
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
