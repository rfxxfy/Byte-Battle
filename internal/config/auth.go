package config

import (
	"strconv"
	"time"
)

type AuthConfig struct {
	SessionTTL          time.Duration // время жизни сессии
	VerificationCodeTTL time.Duration // время жизни кода подтверждения
	MaxVerifyAttempts   int           // макс попыток ввода кода
	BcryptCost          int           // стоимость bcrypt
	CookieName          string        // имя cookie для сессии
	CookieSecure        bool          // secure flag для cookie
}

func LoadAuthConfig() *AuthConfig {
	sessionTTLHours, _ := strconv.Atoi(getEnv("SESSION_TTL_HOURS", "168"))
	verifyTTLMinutes, _ := strconv.Atoi(getEnv("VERIFY_CODE_TTL_MINUTES", "15"))
	maxAttempts, _ := strconv.Atoi(getEnv("VERIFY_MAX_ATTEMPTS", "5"))
	bcryptCost, _ := strconv.Atoi(getEnv("BCRYPT_COST", "10"))
	cookieSecure := getEnv("COOKIE_SECURE", "false") == "true"

	return &AuthConfig{
		SessionTTL:          time.Duration(sessionTTLHours) * time.Hour,
		VerificationCodeTTL: time.Duration(verifyTTLMinutes) * time.Minute,
		MaxVerifyAttempts:   maxAttempts,
		BcryptCost:          bcryptCost,
		CookieName:          getEnv("COOKIE_NAME", "bb_session"),
		CookieSecure:        cookieSecure,
	}
}