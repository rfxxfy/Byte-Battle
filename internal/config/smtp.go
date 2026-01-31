package config

import "strconv"

type SMTPConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	From     string
	Enabled  bool
}

func LoadSMTPConfig() *SMTPConfig {
	host := getEnv("SMTP_HOST", "")
	port, _ := strconv.Atoi(getEnv("SMTP_PORT", "587"))

	return &SMTPConfig{
		Host:     host,
		Port:     port,
		User:     getEnv("SMTP_USER", ""),
		Password: getEnv("SMTP_PASSWORD", ""),
		From:     getEnv("SMTP_FROM", "noreply@bytebattle.local"),
		Enabled:  host != "",
	}
}