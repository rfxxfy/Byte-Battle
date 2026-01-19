package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"time"

	"bytebattle/internal/config"
	sqlcdb "bytebattle/internal/db/sqlc"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/crypto/bcrypt"
)

// Sentinel errors.
var (
	ErrInvalidEmail    = errors.New("invalid email format")
	ErrUserNotFound    = errors.New("user not found")
	ErrInvalidCode     = errors.New("invalid or expired verification code")
	ErrTooManyAttempts = errors.New("too many verification attempts")
)

// EntranceService handles the single email-based entrance flow.
// It transparently registers new users and authenticates existing ones —
// the caller never needs to know which case occurred.
type EntranceService interface {
	// SendCode creates the user if they don't exist yet, then sends a
	// 6-digit verification code to the provided email.
	SendCode(ctx context.Context, email string) error

	// VerifyCode checks the code, marks the email as verified, and returns
	// a session token ready to be stored in a cookie.
	VerifyCode(ctx context.Context, email, code string) (sessionToken string, err error)
}

// entranceDB is the narrow DB interface used by entranceService.
// *sqlcdb.Queries satisfies it automatically.
type entranceDB interface {
	GetUserByEmail(ctx context.Context, email string) (sqlcdb.User, error)
	GetUserByUsername(ctx context.Context, username string) (sqlcdb.User, error)
	CreateUserByEmail(ctx context.Context, arg sqlcdb.CreateUserByEmailParams) (sqlcdb.User, error)
	GetVerificationCodeByUserID(ctx context.Context, userID int32) (sqlcdb.EmailVerificationCode, error)
	UpsertVerificationCode(ctx context.Context, arg sqlcdb.UpsertVerificationCodeParams) (sqlcdb.EmailVerificationCode, error)
	IncrementVerificationAttempts(ctx context.Context, id int32) error
	DeleteVerificationCode(ctx context.Context, id int32) error
	SetEmailVerified(ctx context.Context, id int32) error
}

// sessionCreator is the narrow session interface used by entranceService.
// *SessionService satisfies it automatically.
type sessionCreator interface {
	CreateSession(ctx context.Context, userID int) (sqlcdb.Session, error)
}

// entranceService is the concrete implementation of EntranceService.
type entranceService struct {
	db      entranceDB
	session sessionCreator
	mailer  Mailer
	cfg     config.EntranceConfig
}

func NewEntranceService(
	db entranceDB,
	session sessionCreator,
	mailer Mailer,
	cfg config.EntranceConfig,
) EntranceService {
	return &entranceService{
		db:      db,
		session: session,
		mailer:  mailer,
		cfg:     cfg,
	}
}

// SendCode implements EntranceService.
func (s *entranceService) SendCode(ctx context.Context, email string) error {
	if !isValidEmail(email) {
		return ErrInvalidEmail
	}

	user, err := s.db.GetUserByEmail(ctx, email)
	if errors.Is(err, pgx.ErrNoRows) {
		// New user — create with auto-generated username.
		username, genErr := s.generateUniqueUsername(ctx)
		if genErr != nil {
			return fmt.Errorf("generate username: %w", genErr)
		}
		user, err = s.db.CreateUserByEmail(ctx, sqlcdb.CreateUserByEmailParams{
			Username: username,
			Email:    email,
		})
		if err != nil {
			return fmt.Errorf("create user: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("get user: %w", err)
	}

	return s.sendCode(ctx, user)
}

// VerifyCode implements EntranceService.
func (s *entranceService) VerifyCode(ctx context.Context, email, code string) (string, error) {
	user, err := s.db.GetUserByEmail(ctx, email)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrUserNotFound
	}
	if err != nil {
		return "", fmt.Errorf("get user: %w", err)
	}

	vc, err := s.db.GetVerificationCodeByUserID(ctx, user.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrInvalidCode
	}
	if err != nil {
		return "", fmt.Errorf("get verification code: %w", err)
	}

	if vc.Attempts >= int32(s.cfg.MaxAttempts) {
		return "", ErrTooManyAttempts
	}

	if time.Now().After(vc.ExpiresAt.Time) {
		return "", ErrInvalidCode
	}

	if err := bcrypt.CompareHashAndPassword([]byte(vc.CodeHash), []byte(code)); err != nil {
		_ = s.db.IncrementVerificationAttempts(ctx, vc.ID)
		return "", ErrInvalidCode
	}

	if err := s.db.SetEmailVerified(ctx, user.ID); err != nil {
		return "", fmt.Errorf("set email verified: %w", err)
	}

	_ = s.db.DeleteVerificationCode(ctx, vc.ID)

	session, err := s.session.CreateSession(ctx, int(user.ID))
	if err != nil {
		return "", fmt.Errorf("create session: %w", err)
	}

	return session.Token, nil
}

// --- helpers ---

func (s *entranceService) sendCode(ctx context.Context, user sqlcdb.User) error {
	code, err := generateVerificationCode()
	if err != nil {
		return fmt.Errorf("generate code: %w", err)
	}

	codeHash, err := bcrypt.GenerateFromPassword([]byte(code), s.cfg.BcryptCost)
	if err != nil {
		return fmt.Errorf("hash code: %w", err)
	}

	expiresAt := time.Now().Add(s.cfg.CodeTTL)
	if _, err := s.db.UpsertVerificationCode(ctx, sqlcdb.UpsertVerificationCodeParams{
		UserID:    user.ID,
		CodeHash:  string(codeHash),
		ExpiresAt: pgtype.Timestamptz{Time: expiresAt.UTC(), Valid: true},
	}); err != nil {
		return fmt.Errorf("save verification code: %w", err)
	}

	if err := s.mailer.SendVerificationCode(user.Email, code); err != nil {
		return fmt.Errorf("send email: %w", err)
	}

	return nil
}

func (s *entranceService) generateUniqueUsername(ctx context.Context) (string, error) {
	for i := 0; i < 10; i++ {
		b := make([]byte, 4)
		if _, err := rand.Read(b); err != nil {
			return "", err
		}
		username := "user_" + hex.EncodeToString(b)
		if _, err := s.db.GetUserByUsername(ctx, username); errors.Is(err, pgx.ErrNoRows) {
			return username, nil
		}
	}
	return "", errors.New("failed to generate unique username")
}

func generateVerificationCode() (string, error) {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	num := (uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])) % 1_000_000
	return fmt.Sprintf("%06d", num), nil
}

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

func isValidEmail(email string) bool {
	return emailRegex.MatchString(email)
}
