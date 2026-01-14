package config

import "fmt"

type HTTPConfig struct {
	Host string
	Port string
}

func LoadHTTPConfig() *HTTPConfig {
	return &HTTPConfig{
		Host: getEnv("HTTP_HOST", "0.0.0.0"),
		Port: getEnv("HTTP_PORT", "8080"),
	}
}

func (c *HTTPConfig) Address() string {
	return fmt.Sprintf("%s:%s", c.Host, c.Port)
}
