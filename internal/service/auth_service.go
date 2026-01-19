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

var (
	ErrInvalidEmail    = errors.New("invalid email format")
	ErrUserNotFound    = errors.New("user not found")
	ErrInvalidCode     = errors.New("invalid or expired verification code")
	ErrTooManyAttempts = errors.New("too many verification attempts")
)

// EntranceService handles both registration and login through a single email-code flow.
// The caller never needs to know which case occurred.
type EntranceService interface {
	SendCode(ctx context.Context, email string) error
	VerifyCode(ctx context.Context, email, code string) (sessionToken string, err error)
}

type entranceDB interface {
	GetUserByEmail(ctx context.Context, email string) (sqlcdb.User, error)
	GetUserByUsername(ctx context.Context, username string) (sqlcdb.User, error)
	CreateUserByEmail(ctx context.Context, arg sqlcdb.CreateUserByEmailParams) (sqlcdb.User, error)
	GetVerificationCodeByUserID(ctx context.Context, userID int32) (sqlcdb.EmailVerificationCode, error)
	UpsertVerificationCode(ctx context.Context, arg sqlcdb.UpsertVerificationCodeParams) (sqlcdb.EmailVerificationCode, error)
	IncrementAttemptsIfBelowLimit(ctx context.Context, arg sqlcdb.IncrementAttemptsIfBelowLimitParams) (sqlcdb.EmailVerificationCode, error)
	DeleteVerificationCode(ctx context.Context, id int32) error
	SetEmailVerified(ctx context.Context, id int32) error
}

type sessionCreator interface {
	CreateSession(ctx context.Context, userID int) (sqlcdb.Session, error)
}

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

func (s *entranceService) SendCode(ctx context.Context, email string) error {
	if !isValidEmail(email) {
		return ErrInvalidEmail
	}

	user, err := s.db.GetUserByEmail(ctx, email)
	if errors.Is(err, pgx.ErrNoRows) {
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

	if time.Now().After(vc.ExpiresAt.Time) {
		return "", ErrInvalidCode
	}

	// Atomically increment attempts and check the limit in one query.
	// If no rows returned, the limit was already reached.
	vc, err = s.db.IncrementAttemptsIfBelowLimit(ctx, sqlcdb.IncrementAttemptsIfBelowLimitParams{
		ID:       vc.ID,
		Attempts: int32(s.cfg.MaxAttempts),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrTooManyAttempts
	}
	if err != nil {
		return "", fmt.Errorf("increment attempts: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(vc.CodeHash), []byte(code)); err != nil {
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
