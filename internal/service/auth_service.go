package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"regexp"
	"time"

	"bytebattle/internal/config"
	sqlcdb "bytebattle/internal/db/sqlc"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidEmail     = errors.New("invalid email format")
	ErrInvalidCode      = errors.New("invalid or expired verification code")
	ErrTooManyAttempts  = errors.New("too many verification attempts")
	ErrCodeRecentlySent = errors.New("a code was recently sent, please wait before requesting a new one")
)

// EntranceService handles both registration and login through a single email-code flow.
// The caller never needs to know which case occurred.
type EntranceService interface {
	SendCode(ctx context.Context, email string) error
	VerifyCode(ctx context.Context, email, code string) (sqlcdb.Session, error)
}

type entranceDB interface {
	GetUserByEmail(ctx context.Context, email string) (sqlcdb.User, error)
	GetUserByUsername(ctx context.Context, username string) (sqlcdb.User, error)
	CreateUserByEmail(ctx context.Context, arg sqlcdb.CreateUserByEmailParams) (sqlcdb.User, error)
	SetEmailVerified(ctx context.Context, id uuid.UUID) error
	GetVerificationCode(ctx context.Context, email string) (sqlcdb.VerificationCode, error)
	UpsertVerificationCode(ctx context.Context, arg sqlcdb.UpsertVerificationCodeParams) (sqlcdb.VerificationCode, error)
	IncrementAttemptsIfBelowLimit(ctx context.Context, arg sqlcdb.IncrementAttemptsIfBelowLimitParams) (sqlcdb.VerificationCode, error)
	DeleteVerificationCode(ctx context.Context, email string) error
}

type sessionCreator interface {
	CreateSession(ctx context.Context, userID uuid.UUID) (sqlcdb.Session, error)
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

const sendCooldown = 60 * time.Second

func (s *entranceService) SendCode(ctx context.Context, email string) error {
	if !isValidEmail(email) {
		return ErrInvalidEmail
	}

	if existing, err := s.db.GetVerificationCode(ctx, email); err == nil {
		cooldownThreshold := s.cfg.CodeTTL - sendCooldown
		if cooldownThreshold < 0 {
			cooldownThreshold = 0
		}
		if time.Until(existing.ExpiresAt.Time) > cooldownThreshold {
			return ErrCodeRecentlySent
		}
	}

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
		Email:     email,
		CodeHash:  string(codeHash),
		ExpiresAt: pgtype.Timestamptz{Time: expiresAt.UTC(), Valid: true},
	}); err != nil {
		return fmt.Errorf("save verification code: %w", err)
	}

	if err := s.mailer.SendVerificationCode(ctx, email, code); err != nil {
		if delErr := s.db.DeleteVerificationCode(ctx, email); delErr != nil {
			log.Printf("warn: failed to delete verification code after send error for %s: %v", email, delErr)
		}
		return fmt.Errorf("send email: %w", err)
	}

	return nil
}

func (s *entranceService) VerifyCode(ctx context.Context, email, code string) (sqlcdb.Session, error) {
	vc, err := s.db.GetVerificationCode(ctx, email)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlcdb.Session{}, ErrInvalidCode
	}
	if err != nil {
		return sqlcdb.Session{}, fmt.Errorf("get verification code: %w", err)
	}

	if time.Now().After(vc.ExpiresAt.Time) {
		return sqlcdb.Session{}, ErrInvalidCode
	}

	vc, err = s.db.IncrementAttemptsIfBelowLimit(ctx, sqlcdb.IncrementAttemptsIfBelowLimitParams{
		Email:    email,
		Attempts: int32(s.cfg.MaxAttempts),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlcdb.Session{}, ErrTooManyAttempts
	}
	if err != nil {
		return sqlcdb.Session{}, fmt.Errorf("increment attempts: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(vc.CodeHash), []byte(code)); err != nil {
		return sqlcdb.Session{}, ErrInvalidCode
	}

	user, err := s.getOrCreateUser(ctx, email)
	if err != nil {
		return sqlcdb.Session{}, err
	}

	if err := s.db.SetEmailVerified(ctx, user.ID); err != nil {
		return sqlcdb.Session{}, fmt.Errorf("set email verified: %w", err)
	}

	if err := s.db.DeleteVerificationCode(ctx, email); err != nil {
		log.Printf("warn: failed to delete verification code for %s: %v", email, err)
	}

	session, err := s.session.CreateSession(ctx, user.ID)
	if err != nil {
		return sqlcdb.Session{}, fmt.Errorf("create session: %w", err)
	}

	return session, nil
}

func (s *entranceService) getOrCreateUser(ctx context.Context, email string) (sqlcdb.User, error) {
	user, err := s.db.GetUserByEmail(ctx, email)
	if err == nil {
		return user, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return sqlcdb.User{}, fmt.Errorf("get user: %w", err)
	}

	for range 10 {
		username, genErr := s.generateUniqueUsername(ctx)
		if genErr != nil {
			return sqlcdb.User{}, fmt.Errorf("generate username: %w", genErr)
		}

		user, err = s.db.CreateUserByEmail(ctx, sqlcdb.CreateUserByEmailParams{
			Username: username,
			Email:    email,
		})
		if err == nil {
			return user, nil
		}

		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
			return sqlcdb.User{}, fmt.Errorf("create user: %w", err)
		}

		switch pgErr.ConstraintName {
		case "users_email_key":
			// Another request registered this email concurrently.
			user, err = s.db.GetUserByEmail(ctx, email)
			if err != nil {
				return sqlcdb.User{}, fmt.Errorf("get user after email conflict: %w", err)
			}
			return user, nil
		case "users_username_key":
			// Random username collided; retry with a new one.
			continue
		default:
			return sqlcdb.User{}, fmt.Errorf("create user: %w", err)
		}
	}

	return sqlcdb.User{}, errors.New("failed to create user after repeated username conflicts")
}

func (s *entranceService) generateUniqueUsername(ctx context.Context) (string, error) {
	for range 10 {
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
