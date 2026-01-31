package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"time"

	"bytebattle/internal/config"
	"bytebattle/internal/database"
	"bytebattle/internal/database/models"

	"golang.org/x/crypto/bcrypt"
)

// Ошибки аутентификации
var (
	ErrInvalidEmail        = errors.New("invalid email format")
	ErrPasswordTooShort    = errors.New("password must be at least 8 characters")
	ErrEmailAlreadyExists  = errors.New("email already registered and verified")
	ErrUserNotFound        = errors.New("user not found")
	ErrInvalidCredentials  = errors.New("invalid email or password")
	ErrEmailNotVerified    = errors.New("email not verified")
	ErrInvalidCode         = errors.New("invalid or expired verification code")
	ErrTooManyAttempts     = errors.New("too many verification attempts")
	ErrSessionNotFound     = errors.New("session not found or expired")
)

type AuthService struct {
	users        database.UserRepository
	verification database.VerificationRepository
	sessions     database.SessionRepository
	mailer       Mailer
	cfg          *config.AuthConfig
}

func NewAuthService(
	users database.UserRepository,
	verification database.VerificationRepository,
	sessions database.SessionRepository,
	mailer Mailer,
	cfg *config.AuthConfig,
) *AuthService {
	return &AuthService{
		users:        users,
		verification: verification,
		sessions:     sessions,
		mailer:       mailer,
		cfg:          cfg,
	}
}

// --- Registration ---

type RegisterResult struct {
	UserID int
	IsNew  bool
}

func (s *AuthService) Register(ctx context.Context, email, password string) (*RegisterResult, error) {
	// Валидация
	if !isValidEmail(email) {
		return nil, ErrInvalidEmail
	}
	if len(password) < 8 {
		return nil, ErrPasswordTooShort
	}

	// Проверяем существующего пользователя
	existingUser, err := s.users.GetByEmail(ctx, email)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("check existing user: %w", err)
	}

	var user *models.User
	var isNew bool

	if existingUser != nil {
		// Пользователь существует
		if existingUser.EmailVerified {
			return nil, ErrEmailAlreadyExists
		}
		// Не подтверждён — обновляем пароль и переотправляем код
		passwordHash, err := s.hashPassword(password)
		if err != nil {
			return nil, fmt.Errorf("hash password: %w", err)
		}
		if err := s.users.UpdatePasswordHash(ctx, existingUser.ID, passwordHash); err != nil {
			return nil, fmt.Errorf("update password: %w", err)
		}
		user = existingUser
		isNew = false
	} else {
		// Новый пользователь
		passwordHash, err := s.hashPassword(password)
		if err != nil {
			return nil, fmt.Errorf("hash password: %w", err)
		}

		username, err := s.generateUniqueUsername(ctx)
		if err != nil {
			return nil, fmt.Errorf("generate username: %w", err)
		}

		user, err = s.users.Create(ctx, username, email, passwordHash)
		if err != nil {
			return nil, fmt.Errorf("create user: %w", err)
		}
		isNew = true
	}

	// Генерируем и отправляем код
	if err := s.sendVerificationCode(ctx, user); err != nil {
		return nil, fmt.Errorf("send verification code: %w", err)
	}

	return &RegisterResult{UserID: user.ID, IsNew: isNew}, nil
}

// --- Email Confirmation ---

type ConfirmResult struct {
	UserID      int
	SessionToken string
}

func (s *AuthService) ConfirmEmail(ctx context.Context, email, code string) (*ConfirmResult, error) {
	user, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("get user: %w", err)
	}

	verification, err := s.verification.GetByUserID(ctx, user.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrInvalidCode
		}
		return nil, fmt.Errorf("get verification: %w", err)
	}

	// Проверяем попытки
	if verification.Attempts >= s.cfg.MaxVerifyAttempts {
		return nil, ErrTooManyAttempts
	}

	// Проверяем срок действия
	if time.Now().After(verification.ExpiresAt) {
		return nil, ErrInvalidCode
	}

	// Проверяем код
	if err := bcrypt.CompareHashAndPassword([]byte(verification.CodeHash), []byte(code)); err != nil {
		// Увеличиваем счётчик попыток
		_ = s.verification.IncrementAttempts(ctx, verification.ID)
		return nil, ErrInvalidCode
	}

	// Подтверждаем email
	if err := s.users.SetEmailVerified(ctx, user.ID, true); err != nil {
		return nil, fmt.Errorf("set email verified: %w", err)
	}

	// Удаляем код верификации
	_ = s.verification.Delete(ctx, verification.ID)

	// Создаём сессию
	token, err := s.createSession(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	return &ConfirmResult{UserID: user.ID, SessionToken: token}, nil
}

// --- Login ---

type LoginResult struct {
	UserID       int
	SessionToken string
}

func (s *AuthService) Login(ctx context.Context, email, password string) (*LoginResult, error) {
	user, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("get user: %w", err)
	}

	// Проверяем подтверждение email
	if !user.EmailVerified {
		return nil, ErrEmailNotVerified
	}

	// Проверяем пароль
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	// Создаём сессию
	token, err := s.createSession(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	return &LoginResult{UserID: user.ID, SessionToken: token}, nil
}

// --- Logout ---

func (s *AuthService) Logout(ctx context.Context, sessionToken string) error {
	tokenHash := s.hashToken(sessionToken)
	return s.sessions.Revoke(ctx, tokenHash)
}

// --- Session Validation ---

func (s *AuthService) ValidateSession(ctx context.Context, sessionToken string) (*models.User, error) {
	tokenHash := s.hashToken(sessionToken)

	session, err := s.sessions.GetByTokenHash(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrSessionNotFound
		}
		return nil, fmt.Errorf("get session: %w", err)
	}

	user, err := s.users.GetByID(ctx, session.UserID)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}

	return user, nil
}

// --- Helpers ---

func (s *AuthService) hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), s.cfg.BcryptCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func (s *AuthService) hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

func (s *AuthService) generateVerificationCode() (string, error) {
	bytes := make([]byte, 4)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	num := uint32(bytes[0])<<24 | uint32(bytes[1])<<16 | uint32(bytes[2])<<8 | uint32(bytes[3])
	code := num % 1000000
	return fmt.Sprintf("%06d", code), nil
}

func (s *AuthService) sendVerificationCode(ctx context.Context, user *models.User) error {
	code, err := s.generateVerificationCode()
	if err != nil {
		return fmt.Errorf("generate code: %w", err)
	}

	codeHash, err := bcrypt.GenerateFromPassword([]byte(code), s.cfg.BcryptCost)
	if err != nil {
		return fmt.Errorf("hash code: %w", err)
	}

	expiresAt := time.Now().Add(s.cfg.VerificationCodeTTL)

	if _, err := s.verification.Upsert(ctx, user.ID, string(codeHash), expiresAt); err != nil {
		return fmt.Errorf("save verification: %w", err)
	}

	if err := s.mailer.SendVerificationCode(user.Email, code); err != nil {
		return fmt.Errorf("send email: %w", err)
	}

	return nil
}

func (s *AuthService) createSession(ctx context.Context, userID int) (string, error) {
	// Генерируем случайный токен
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	token := hex.EncodeToString(tokenBytes)

	tokenHash := s.hashToken(token)
	expiresAt := time.Now().Add(s.cfg.SessionTTL)

	if _, err := s.sessions.Create(ctx, userID, tokenHash, expiresAt); err != nil {
		return "", fmt.Errorf("save session: %w", err)
	}

	return token, nil
}

func (s *AuthService) generateUniqueUsername(ctx context.Context) (string, error) {
	const maxAttempts = 10

	for i := 0; i < maxAttempts; i++ {
		bytes := make([]byte, 4)
		if _, err := rand.Read(bytes); err != nil {
			return "", err
		}
		username := fmt.Sprintf("user_%s", hex.EncodeToString(bytes))

		_, err := s.users.GetByUsername(ctx, username)
		if errors.Is(err, sql.ErrNoRows) {
			return username, nil
		}
		if err != nil {
			return "", err
		}
		// Username exists, retry
	}

	return "", errors.New("failed to generate unique username")
}

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

func isValidEmail(email string) bool {
	return emailRegex.MatchString(email)
}