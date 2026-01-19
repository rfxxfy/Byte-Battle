package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"bytebattle/internal/config"
	sqlcdb "bytebattle/internal/db/sqlc"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/crypto/bcrypt"
)

// --- test helpers ---

func testEntranceConfig() config.EntranceConfig {
	return config.EntranceConfig{
		CodeTTL:     15 * time.Minute,
		MaxAttempts: 5,
		BcryptCost:  4, // low cost for fast tests
		SessionTTL:  24 * time.Hour,
		CookieName:  "bb_session",
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

func validCode(t *testing.T, expiresAt time.Time, attempts int32) sqlcdb.EmailVerificationCode {
	t.Helper()
	return sqlcdb.EmailVerificationCode{
		ID:        1,
		UserID:    1,
		CodeHash:  hashCode(t, "111111"),
		ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
		Attempts:  attempts,
	}
}

// --- mock entranceDB ---

type mockDB struct {
	getUserByEmail          func(context.Context, string) (sqlcdb.User, error)
	getUserByUsername       func(context.Context, string) (sqlcdb.User, error)
	createUserByEmail       func(context.Context, sqlcdb.CreateUserByEmailParams) (sqlcdb.User, error)
	getVerificationByUserID func(context.Context, int32) (sqlcdb.EmailVerificationCode, error)
	upsertVerification      func(context.Context, sqlcdb.UpsertVerificationCodeParams) (sqlcdb.EmailVerificationCode, error)
	incrementAttempts       func(context.Context, int32) error
	deleteVerification      func(context.Context, int32) error
	setEmailVerified        func(context.Context, int32) error

	upsertCalled         bool
	incrementCalled      bool
	setVerifiedCalled    bool
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
	return sqlcdb.User{ID: 1, Username: arg.Username, Email: arg.Email}, nil
}

func (m *mockDB) GetVerificationCodeByUserID(ctx context.Context, userID int32) (sqlcdb.EmailVerificationCode, error) {
	if m.getVerificationByUserID != nil {
		return m.getVerificationByUserID(ctx, userID)
	}
	return sqlcdb.EmailVerificationCode{}, pgx.ErrNoRows
}

func (m *mockDB) UpsertVerificationCode(ctx context.Context, arg sqlcdb.UpsertVerificationCodeParams) (sqlcdb.EmailVerificationCode, error) {
	m.upsertCalled = true
	if m.upsertVerification != nil {
		return m.upsertVerification(ctx, arg)
	}
	return sqlcdb.EmailVerificationCode{ID: 1, UserID: arg.UserID}, nil
}

func (m *mockDB) IncrementVerificationAttempts(ctx context.Context, id int32) error {
	m.incrementCalled = true
	if m.incrementAttempts != nil {
		return m.incrementAttempts(ctx, id)
	}
	return nil
}

func (m *mockDB) DeleteVerificationCode(ctx context.Context, id int32) error {
	if m.deleteVerification != nil {
		return m.deleteVerification(ctx, id)
	}
	return nil
}

func (m *mockDB) SetEmailVerified(ctx context.Context, id int32) error {
	m.setVerifiedCalled = true
	if m.setEmailVerified != nil {
		return m.setEmailVerified(ctx, id)
	}
	return nil
}

// --- mock sessionCreator ---

type mockSession struct {
	token        string
	createCalled bool
	err          error
}

func (m *mockSession) CreateSession(_ context.Context, _ int) (sqlcdb.Session, error) {
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

// --- mock Mailer ---

type mockMailer struct {
	sendCalled bool
	lastTo     string
	err        error
}

func (m *mockMailer) SendVerificationCode(to, _ string) error {
	m.sendCalled = true
	m.lastTo = to
	return m.err
}

// --- factory ---

func newEntrance(db *mockDB, sess *mockSession, mailer *mockMailer) EntranceService {
	return NewEntranceService(db, sess, mailer, testEntranceConfig())
}

// =============================================================================
// Tests: pure functions
// =============================================================================

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

// =============================================================================
// Tests: SendCode
// =============================================================================

func TestSendCode_InvalidEmail_ReturnsError(t *testing.T) {
	db := &mockDB{}
	svc := newEntrance(db, &mockSession{}, &mockMailer{})

	err := svc.SendCode(context.Background(), "not-an-email")

	if !errors.Is(err, ErrInvalidEmail) {
		t.Errorf("expected ErrInvalidEmail, got %v", err)
	}
}

func TestSendCode_NewUser_CreatesAndSendsCode(t *testing.T) {
	db := &mockDB{
		// no existing user
		getUserByEmail: func(_ context.Context, _ string) (sqlcdb.User, error) {
			return sqlcdb.User{}, pgx.ErrNoRows
		},
	}
	mailer := &mockMailer{}
	svc := newEntrance(db, &mockSession{}, mailer)

	err := svc.SendCode(context.Background(), "new@example.com")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !db.upsertCalled {
		t.Error("expected UpsertVerificationCode to be called")
	}
	if !mailer.sendCalled {
		t.Error("expected mailer.SendVerificationCode to be called")
	}
	if mailer.lastTo != "new@example.com" {
		t.Errorf("expected email sent to new@example.com, got %s", mailer.lastTo)
	}
}

func TestSendCode_ExistingUser_SendsCodeWithoutCreating(t *testing.T) {
	createCalled := false
	db := &mockDB{
		getUserByEmail: func(_ context.Context, _ string) (sqlcdb.User, error) {
			return sqlcdb.User{ID: 42, Email: "existing@example.com"}, nil
		},
		createUserByEmail: func(_ context.Context, _ sqlcdb.CreateUserByEmailParams) (sqlcdb.User, error) {
			createCalled = true
			return sqlcdb.User{}, nil
		},
	}
	mailer := &mockMailer{}
	svc := newEntrance(db, &mockSession{}, mailer)

	err := svc.SendCode(context.Background(), "existing@example.com")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if createCalled {
		t.Error("CreateUserByEmail should not be called for existing users")
	}
	if !mailer.sendCalled {
		t.Error("expected mailer to be called")
	}
}

// =============================================================================
// Tests: VerifyCode
// =============================================================================

func TestVerifyCode_Success_ReturnsToken(t *testing.T) {
	db := &mockDB{
		getUserByEmail: func(_ context.Context, _ string) (sqlcdb.User, error) {
			return sqlcdb.User{ID: 1, Email: "user@example.com"}, nil
		},
		getVerificationByUserID: func(_ context.Context, _ int32) (sqlcdb.EmailVerificationCode, error) {
			return validCode(t, time.Now().Add(10*time.Minute), 0), nil
		},
	}
	sess := &mockSession{token: "my-session-token"}
	svc := newEntrance(db, sess, &mockMailer{})

	token, err := svc.VerifyCode(context.Background(), "user@example.com", "111111")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "my-session-token" {
		t.Errorf("expected token 'my-session-token', got %q", token)
	}
	if !db.setVerifiedCalled {
		t.Error("expected SetEmailVerified to be called")
	}
	if !sess.createCalled {
		t.Error("expected CreateSession to be called")
	}
}

func TestVerifyCode_WrongCode_IncrementsAttemptsAndErrors(t *testing.T) {
	db := &mockDB{
		getUserByEmail: func(_ context.Context, _ string) (sqlcdb.User, error) {
			return sqlcdb.User{ID: 1, Email: "user@example.com"}, nil
		},
		getVerificationByUserID: func(_ context.Context, _ int32) (sqlcdb.EmailVerificationCode, error) {
			return validCode(t, time.Now().Add(10*time.Minute), 0), nil
		},
	}
	svc := newEntrance(db, &mockSession{}, &mockMailer{})

	_, err := svc.VerifyCode(context.Background(), "user@example.com", "000000") // wrong code

	if !errors.Is(err, ErrInvalidCode) {
		t.Errorf("expected ErrInvalidCode, got %v", err)
	}
	if !db.incrementCalled {
		t.Error("expected IncrementVerificationAttempts to be called")
	}
	if db.setVerifiedCalled {
		t.Error("SetEmailVerified should NOT be called on wrong code")
	}
}

func TestVerifyCode_TooManyAttempts(t *testing.T) {
	cfg := testEntranceConfig()
	db := &mockDB{
		getUserByEmail: func(_ context.Context, _ string) (sqlcdb.User, error) {
			return sqlcdb.User{ID: 1}, nil
		},
		getVerificationByUserID: func(_ context.Context, _ int32) (sqlcdb.EmailVerificationCode, error) {
			return validCode(t, time.Now().Add(10*time.Minute), int32(cfg.MaxAttempts)), nil
		},
	}
	svc := newEntrance(db, &mockSession{}, &mockMailer{})

	_, err := svc.VerifyCode(context.Background(), "user@example.com", "111111")

	if !errors.Is(err, ErrTooManyAttempts) {
		t.Errorf("expected ErrTooManyAttempts, got %v", err)
	}
	if db.incrementCalled {
		t.Error("IncrementVerificationAttempts should NOT be called at max attempts")
	}
}

func TestVerifyCode_ExpiredCode(t *testing.T) {
	db := &mockDB{
		getUserByEmail: func(_ context.Context, _ string) (sqlcdb.User, error) {
			return sqlcdb.User{ID: 1}, nil
		},
		getVerificationByUserID: func(_ context.Context, _ int32) (sqlcdb.EmailVerificationCode, error) {
			return validCode(t, time.Now().Add(-1*time.Minute), 0), nil // expired
		},
	}
	svc := newEntrance(db, &mockSession{}, &mockMailer{})

	_, err := svc.VerifyCode(context.Background(), "user@example.com", "111111")

	if !errors.Is(err, ErrInvalidCode) {
		t.Errorf("expected ErrInvalidCode for expired code, got %v", err)
	}
}

func TestVerifyCode_UserNotFound(t *testing.T) {
	db := &mockDB{} // getUserByEmail returns pgx.ErrNoRows by default
	svc := newEntrance(db, &mockSession{}, &mockMailer{})

	_, err := svc.VerifyCode(context.Background(), "ghost@example.com", "111111")

	if !errors.Is(err, ErrUserNotFound) {
		t.Errorf("expected ErrUserNotFound, got %v", err)
	}
}
