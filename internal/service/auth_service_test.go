package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"bytebattle/internal/config"
	sqlcdb "bytebattle/internal/db/sqlc"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/crypto/bcrypt"
)

func testEntranceConfig() config.EntranceConfig {
	return config.EntranceConfig{
		CodeTTL:     15 * time.Minute,
		MaxAttempts: 5,
		BcryptCost:  4, // low cost for fast tests
		SessionTTL:  24 * time.Hour,
	}
}

func hashCode(t *testing.T, code string) string {
	t.Helper()
	h, err := bcrypt.GenerateFromPassword([]byte(code), 4)
	if err != nil {
		t.Fatalf("hashCode: %v", err)
	}
	return string(h)
}

func validCode(t *testing.T, expiresAt time.Time, attempts int32) sqlcdb.VerificationCode {
	t.Helper()
	return sqlcdb.VerificationCode{
		Email:     "user@example.com",
		CodeHash:  hashCode(t, "111111"),
		ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
		Attempts:  attempts,
	}
}

type mockDB struct {
	getUserByEmail     func(context.Context, string) (sqlcdb.User, error)
	getUserByUsername  func(context.Context, string) (sqlcdb.User, error)
	createUserByEmail  func(context.Context, sqlcdb.CreateUserByEmailParams) (sqlcdb.User, error)
	getVerification    func(context.Context, string) (sqlcdb.VerificationCode, error)
	upsertVerification func(context.Context, sqlcdb.UpsertVerificationCodeParams) (sqlcdb.VerificationCode, error)
	incrementAttempts  func(context.Context, sqlcdb.IncrementAttemptsIfBelowLimitParams) (sqlcdb.VerificationCode, error)
	deleteVerification func(context.Context, string) error
	setEmailVerified   func(context.Context, uuid.UUID) error

	upsertCalled      bool
	incrementCalled   bool
	setVerifiedCalled bool
}

func (m *mockDB) GetUserByEmail(ctx context.Context, email string) (sqlcdb.User, error) {
	if m.getUserByEmail != nil {
		return m.getUserByEmail(ctx, email)
	}
	return sqlcdb.User{}, pgx.ErrNoRows
}

func (m *mockDB) GetUserByUsername(ctx context.Context, username string) (sqlcdb.User, error) {
	if m.getUserByUsername != nil {
		return m.getUserByUsername(ctx, username)
	}
	return sqlcdb.User{}, pgx.ErrNoRows
}

func (m *mockDB) CreateUserByEmail(ctx context.Context, arg sqlcdb.CreateUserByEmailParams) (sqlcdb.User, error) {
	if m.createUserByEmail != nil {
		return m.createUserByEmail(ctx, arg)
	}
	return sqlcdb.User{ID: uuid.UUID{}, Username: arg.Username, Email: arg.Email}, nil
}

func (m *mockDB) GetVerificationCode(ctx context.Context, email string) (sqlcdb.VerificationCode, error) {
	if m.getVerification != nil {
		return m.getVerification(ctx, email)
	}
	return sqlcdb.VerificationCode{}, pgx.ErrNoRows
}

func (m *mockDB) UpsertVerificationCode(ctx context.Context, arg sqlcdb.UpsertVerificationCodeParams) (sqlcdb.VerificationCode, error) {
	m.upsertCalled = true
	if m.upsertVerification != nil {
		return m.upsertVerification(ctx, arg)
	}
	return sqlcdb.VerificationCode{Email: arg.Email}, nil
}

func (m *mockDB) IncrementAttemptsIfBelowLimit(ctx context.Context, arg sqlcdb.IncrementAttemptsIfBelowLimitParams) (sqlcdb.VerificationCode, error) {
	m.incrementCalled = true
	if m.incrementAttempts != nil {
		return m.incrementAttempts(ctx, arg)
	}
	return sqlcdb.VerificationCode{Email: arg.Email}, nil
}

func (m *mockDB) DeleteVerificationCode(ctx context.Context, email string) error {
	if m.deleteVerification != nil {
		return m.deleteVerification(ctx, email)
	}
	return nil
}

func (m *mockDB) SetEmailVerified(ctx context.Context, id uuid.UUID) error {
	m.setVerifiedCalled = true
	if m.setEmailVerified != nil {
		return m.setEmailVerified(ctx, id)
	}
	return nil
}

type mockSession struct {
	token        string
	createCalled bool
	err          error
}

func (m *mockSession) CreateSession(_ context.Context, _ uuid.UUID) (sqlcdb.Session, error) {
	m.createCalled = true
	if m.err != nil {
		return sqlcdb.Session{}, m.err
	}
	tok := m.token
	if tok == "" {
		tok = "test-token"
	}
	return sqlcdb.Session{ID: 1, Token: tok}, nil
}

type mockMailer struct {
	sendCalled bool
	lastTo     string
	err        error
}

func (m *mockMailer) SendVerificationCode(_ context.Context, to, _ string) error {
	m.sendCalled = true
	m.lastTo = to
	return m.err
}

func newEntrance(db *mockDB, sess *mockSession, mailer *mockMailer) EntranceService {
	return NewEntranceService(db, sess, mailer, testEntranceConfig())
}

func TestIsValidEmail(t *testing.T) {
	valid := []string{"user@example.com", "a+b@x.io", "foo.bar@baz.co.uk"}
	invalid := []string{"notanemail", "@example.com", "foo@", "foo bar@x.com"}

	for _, e := range valid {
		if !isValidEmail(e) {
			t.Errorf("expected %q to be valid", e)
		}
	}
	for _, e := range invalid {
		if isValidEmail(e) {
			t.Errorf("expected %q to be invalid", e)
		}
	}
}

func TestGenerateVerificationCode_Format(t *testing.T) {
	for i := 0; i < 50; i++ {
		code, err := generateVerificationCode()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(code) != 6 {
			t.Errorf("expected 6-digit code, got %q (len=%d)", code, len(code))
		}
		for _, ch := range code {
			if ch < '0' || ch > '9' {
				t.Errorf("non-digit character in code %q", code)
			}
		}
	}
}

func TestSendCode_InvalidEmail_ReturnsError(t *testing.T) {
	svc := newEntrance(&mockDB{}, &mockSession{}, &mockMailer{})

	err := svc.SendCode(context.Background(), "not-an-email")

	if !errors.Is(err, ErrInvalidEmail) {
		t.Errorf("expected ErrInvalidEmail, got %v", err)
	}
}

func TestSendCode_UpsertCodeAndSendEmail(t *testing.T) {
	db := &mockDB{}
	mailer := &mockMailer{}
	svc := newEntrance(db, &mockSession{}, mailer)

	err := svc.SendCode(context.Background(), "user@example.com")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !db.upsertCalled {
		t.Error("expected UpsertVerificationCode to be called")
	}
	if !mailer.sendCalled {
		t.Error("expected mailer.SendVerificationCode to be called")
	}
	if mailer.lastTo != "user@example.com" {
		t.Errorf("expected email sent to user@example.com, got %s", mailer.lastTo)
	}
}

func TestVerifyCode_ExistingUser_ReturnsSession(t *testing.T) {
	code := validCode(t, time.Now().Add(10*time.Minute), 0)
	db := &mockDB{
		getVerification: func(_ context.Context, _ string) (sqlcdb.VerificationCode, error) {
			return code, nil
		},
		incrementAttempts: func(_ context.Context, _ sqlcdb.IncrementAttemptsIfBelowLimitParams) (sqlcdb.VerificationCode, error) {
			return code, nil
		},
		getUserByEmail: func(_ context.Context, _ string) (sqlcdb.User, error) {
			return sqlcdb.User{ID: uuid.UUID{}, Email: "user@example.com"}, nil
		},
	}
	sess := &mockSession{token: "my-session-token"}
	svc := newEntrance(db, sess, &mockMailer{})

	session, err := svc.VerifyCode(context.Background(), "user@example.com", "111111")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if session.Token != "my-session-token" {
		t.Errorf("expected token 'my-session-token', got %q", session.Token)
	}
	if !db.setVerifiedCalled {
		t.Error("expected SetEmailVerified to be called")
	}
	if !sess.createCalled {
		t.Error("expected CreateSession to be called")
	}
}

func TestVerifyCode_NewUser_CreatesUserOnVerify(t *testing.T) {
	code := validCode(t, time.Now().Add(10*time.Minute), 0)
	createCalled := false
	db := &mockDB{
		getVerification: func(_ context.Context, _ string) (sqlcdb.VerificationCode, error) {
			return code, nil
		},
		incrementAttempts: func(_ context.Context, _ sqlcdb.IncrementAttemptsIfBelowLimitParams) (sqlcdb.VerificationCode, error) {
			return code, nil
		},
		// no existing user
		getUserByEmail: func(_ context.Context, _ string) (sqlcdb.User, error) {
			return sqlcdb.User{}, pgx.ErrNoRows
		},
		createUserByEmail: func(_ context.Context, arg sqlcdb.CreateUserByEmailParams) (sqlcdb.User, error) {
			createCalled = true
			return sqlcdb.User{ID: uuid.UUID{}, Email: arg.Email, Username: arg.Username}, nil
		},
	}
	sess := &mockSession{}
	svc := newEntrance(db, sess, &mockMailer{})

	_, err := svc.VerifyCode(context.Background(), "new@example.com", "111111")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !createCalled {
		t.Error("expected CreateUserByEmail to be called for new user")
	}
	if !db.setVerifiedCalled {
		t.Error("expected SetEmailVerified to be called")
	}
	if !sess.createCalled {
		t.Error("expected CreateSession to be called")
	}
}

func TestVerifyCode_WrongCode_IncrementsAttemptsAndErrors(t *testing.T) {
	code := validCode(t, time.Now().Add(10*time.Minute), 0)
	db := &mockDB{
		getVerification: func(_ context.Context, _ string) (sqlcdb.VerificationCode, error) {
			return code, nil
		},
	}
	svc := newEntrance(db, &mockSession{}, &mockMailer{})

	_, err := svc.VerifyCode(context.Background(), "user@example.com", "000000") // wrong code

	if !errors.Is(err, ErrInvalidCode) {
		t.Errorf("expected ErrInvalidCode, got %v", err)
	}
	if !db.incrementCalled {
		t.Error("expected IncrementAttemptsIfBelowLimit to be called")
	}
	if db.setVerifiedCalled {
		t.Error("SetEmailVerified should NOT be called on wrong code")
	}
}

func TestVerifyCode_TooManyAttempts(t *testing.T) {
	code := validCode(t, time.Now().Add(10*time.Minute), 0)
	db := &mockDB{
		getVerification: func(_ context.Context, _ string) (sqlcdb.VerificationCode, error) {
			return code, nil
		},
		incrementAttempts: func(_ context.Context, _ sqlcdb.IncrementAttemptsIfBelowLimitParams) (sqlcdb.VerificationCode, error) {
			return sqlcdb.VerificationCode{}, pgx.ErrNoRows
		},
	}
	svc := newEntrance(db, &mockSession{}, &mockMailer{})

	_, err := svc.VerifyCode(context.Background(), "user@example.com", "111111")

	if !errors.Is(err, ErrTooManyAttempts) {
		t.Errorf("expected ErrTooManyAttempts, got %v", err)
	}
}

func TestVerifyCode_ExpiredCode(t *testing.T) {
	db := &mockDB{
		getVerification: func(_ context.Context, _ string) (sqlcdb.VerificationCode, error) {
			return validCode(t, time.Now().Add(-1*time.Minute), 0), nil // expired
		},
	}
	svc := newEntrance(db, &mockSession{}, &mockMailer{})

	_, err := svc.VerifyCode(context.Background(), "user@example.com", "111111")

	if !errors.Is(err, ErrInvalidCode) {
		t.Errorf("expected ErrInvalidCode for expired code, got %v", err)
	}
}

func TestSendCode_RateLimitsRecentCode(t *testing.T) {
	cfg := testEntranceConfig() // CodeTTL = 15min
	db := &mockDB{
		getVerification: func(_ context.Context, _ string) (sqlcdb.VerificationCode, error) {
			// Code expires in 14.5 min → sent 30s ago, within 60s cooldown
			return sqlcdb.VerificationCode{
				Email:     "user@example.com",
				ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(cfg.CodeTTL - 30*time.Second), Valid: true},
			}, nil
		},
	}
	svc := NewEntranceService(db, &mockSession{}, &mockMailer{}, cfg)

	err := svc.SendCode(context.Background(), "user@example.com")

	if !errors.Is(err, ErrCodeRecentlySent) {
		t.Errorf("expected ErrCodeRecentlySent, got %v", err)
	}
	if db.upsertCalled {
		t.Error("should not upsert when rate-limited")
	}
}

func TestSendCode_AllowsAfterCooldown(t *testing.T) {
	cfg := testEntranceConfig() // CodeTTL = 15min
	db := &mockDB{
		getVerification: func(_ context.Context, _ string) (sqlcdb.VerificationCode, error) {
			// Code expires in 13 min → sent 2min ago, past 60s cooldown
			return sqlcdb.VerificationCode{
				Email:     "user@example.com",
				ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(cfg.CodeTTL - 2*time.Minute), Valid: true},
			}, nil
		},
	}
	mailer := &mockMailer{}
	svc := NewEntranceService(db, &mockSession{}, mailer, cfg)

	err := svc.SendCode(context.Background(), "user@example.com")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !db.upsertCalled {
		t.Error("expected UpsertVerificationCode to be called after cooldown")
	}
}

func TestVerifyCode_NoCode_ReturnsInvalidCode(t *testing.T) {
	db := &mockDB{} // GetVerificationCode returns pgx.ErrNoRows by default
	svc := newEntrance(db, &mockSession{}, &mockMailer{})

	_, err := svc.VerifyCode(context.Background(), "ghost@example.com", "111111")

	if !errors.Is(err, ErrInvalidCode) {
		t.Errorf("expected ErrInvalidCode, got %v", err)
	}
}

func TestSendCode_EmailFailure_DeletesVerificationCode(t *testing.T) {
	deleted := false
	db := &mockDB{
		deleteVerification: func(_ context.Context, _ string) error {
			deleted = true
			return nil
		},
	}
	mailer := &mockMailer{err: errors.New("smtp error")}
	svc := newEntrance(db, &mockSession{}, mailer)

	err := svc.SendCode(context.Background(), "user@example.com")

	if err == nil {
		t.Fatal("expected error from mailer")
	}
	if !deleted {
		t.Error("expected DeleteVerificationCode to be called after email failure")
	}
}

func TestVerifyCode_UsernameConflict_RetriesWithNewUsername(t *testing.T) {
	code := validCode(t, time.Now().Add(10*time.Minute), 0)
	attempts := 0
	db := &mockDB{
		getVerification: func(_ context.Context, _ string) (sqlcdb.VerificationCode, error) {
			return code, nil
		},
		incrementAttempts: func(_ context.Context, _ sqlcdb.IncrementAttemptsIfBelowLimitParams) (sqlcdb.VerificationCode, error) {
			return code, nil
		},
		getUserByEmail: func(_ context.Context, _ string) (sqlcdb.User, error) {
			return sqlcdb.User{}, pgx.ErrNoRows
		},
		createUserByEmail: func(_ context.Context, arg sqlcdb.CreateUserByEmailParams) (sqlcdb.User, error) {
			attempts++
			if attempts < 3 {
				return sqlcdb.User{}, &pgconn.PgError{Code: "23505", ConstraintName: "users_username_key"}
			}
			return sqlcdb.User{ID: uuid.UUID{}, Email: arg.Email, Username: arg.Username}, nil
		},
	}
	svc := newEntrance(db, &mockSession{}, &mockMailer{})

	_, err := svc.VerifyCode(context.Background(), "new@example.com", "111111")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if attempts != 3 {
		t.Errorf("expected 3 create attempts (2 conflicts + 1 success), got %d", attempts)
	}
}
