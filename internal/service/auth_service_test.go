package service

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"bytebattle/internal/config"
	"bytebattle/internal/database/models"

	"golang.org/x/crypto/bcrypt"
)

// --- Test Config ---

func testAuthConfig() *config.AuthConfig {
	return &config.AuthConfig{
		SessionTTL:          24 * time.Hour,
		VerificationCodeTTL: 15 * time.Minute,
		MaxVerifyAttempts:   5,
		BcryptCost:          4, // Низкий cost для быстрых тестов
		CookieName:          "bb_session",
		CookieSecure:        false,
	}
}

// --- Mock UserRepository ---

type mockUserRepo struct {
	getByID            func(ctx context.Context, id int) (*models.User, error)
	getByUsername      func(ctx context.Context, username string) (*models.User, error)
	getByEmail         func(ctx context.Context, email string) (*models.User, error)
	create             func(ctx context.Context, username, email, passwordHash string) (*models.User, error)
	setEmailVerified   func(ctx context.Context, userID int, verified bool) error
	updatePasswordHash func(ctx context.Context, userID int, passwordHash string) error
}

func (m *mockUserRepo) GetByID(ctx context.Context, id int) (*models.User, error) {
	if m.getByID != nil {
		return m.getByID(ctx, id)
	}
	return nil, sql.ErrNoRows
}

func (m *mockUserRepo) GetByUsername(ctx context.Context, username string) (*models.User, error) {
	if m.getByUsername != nil {
		return m.getByUsername(ctx, username)
	}
	return nil, sql.ErrNoRows
}

func (m *mockUserRepo) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	if m.getByEmail != nil {
		return m.getByEmail(ctx, email)
	}
	return nil, sql.ErrNoRows
}

func (m *mockUserRepo) Create(ctx context.Context, username, email, passwordHash string) (*models.User, error) {
	if m.create != nil {
		return m.create(ctx, username, email, passwordHash)
	}
	return &models.User{ID: 1, Username: username, Email: email, PasswordHash: passwordHash}, nil
}

func (m *mockUserRepo) SetEmailVerified(ctx context.Context, userID int, verified bool) error {
	if m.setEmailVerified != nil {
		return m.setEmailVerified(ctx, userID, verified)
	}
	return nil
}

func (m *mockUserRepo) UpdatePasswordHash(ctx context.Context, userID int, passwordHash string) error {
	if m.updatePasswordHash != nil {
		return m.updatePasswordHash(ctx, userID, passwordHash)
	}
	return nil
}

// --- Mock VerificationRepository ---

type mockVerificationRepo struct {
	upsert            func(ctx context.Context, userID int, codeHash string, expiresAt time.Time) (*models.EmailVerificationCode, error)
	getByUserID       func(ctx context.Context, userID int) (*models.EmailVerificationCode, error)
	incrementAttempts func(ctx context.Context, id int) error
	delete            func(ctx context.Context, id int) error
	deleteByUserID    func(ctx context.Context, userID int) error

	// Счётчики вызовов для проверки
	incrementAttemptsCalled bool
}

func (m *mockVerificationRepo) Upsert(ctx context.Context, userID int, codeHash string, expiresAt time.Time) (*models.EmailVerificationCode, error) {
	if m.upsert != nil {
		return m.upsert(ctx, userID, codeHash, expiresAt)
	}
	return &models.EmailVerificationCode{ID: 1, UserID: userID, CodeHash: codeHash, ExpiresAt: expiresAt}, nil
}

func (m *mockVerificationRepo) GetByUserID(ctx context.Context, userID int) (*models.EmailVerificationCode, error) {
	if m.getByUserID != nil {
		return m.getByUserID(ctx, userID)
	}
	return nil, sql.ErrNoRows
}

func (m *mockVerificationRepo) IncrementAttempts(ctx context.Context, id int) error {
	m.incrementAttemptsCalled = true
	if m.incrementAttempts != nil {
		return m.incrementAttempts(ctx, id)
	}
	return nil
}

func (m *mockVerificationRepo) Delete(ctx context.Context, id int) error {
	if m.delete != nil {
		return m.delete(ctx, id)
	}
	return nil
}

func (m *mockVerificationRepo) DeleteByUserID(ctx context.Context, userID int) error {
	if m.deleteByUserID != nil {
		return m.deleteByUserID(ctx, userID)
	}
	return nil
}

// --- Mock SessionRepository ---

type mockSessionRepo struct {
	create         func(ctx context.Context, userID int, tokenHash string, expiresAt time.Time) (*models.AuthSession, error)
	getByTokenHash func(ctx context.Context, tokenHash string) (*models.AuthSession, error)
	revoke         func(ctx context.Context, tokenHash string) error
	deleteExpired  func(ctx context.Context) (int64, error)
	deleteByUserID func(ctx context.Context, userID int) error

	// Счётчики вызовов для проверки
	createCalled bool
}

func (m *mockSessionRepo) Create(ctx context.Context, userID int, tokenHash string, expiresAt time.Time) (*models.AuthSession, error) {
	m.createCalled = true
	if m.create != nil {
		return m.create(ctx, userID, tokenHash, expiresAt)
	}
	return &models.AuthSession{ID: 1, UserID: userID, TokenHash: tokenHash, ExpiresAt: expiresAt}, nil
}

func (m *mockSessionRepo) GetByTokenHash(ctx context.Context, tokenHash string) (*models.AuthSession, error) {
	if m.getByTokenHash != nil {
		return m.getByTokenHash(ctx, tokenHash)
	}
	return nil, sql.ErrNoRows
}

func (m *mockSessionRepo) Revoke(ctx context.Context, tokenHash string) error {
	if m.revoke != nil {
		return m.revoke(ctx, tokenHash)
	}
	return nil
}

func (m *mockSessionRepo) DeleteExpired(ctx context.Context) (int64, error) {
	if m.deleteExpired != nil {
		return m.deleteExpired(ctx)
	}
	return 0, nil
}

func (m *mockSessionRepo) DeleteByUserID(ctx context.Context, userID int) error {
	if m.deleteByUserID != nil {
		return m.deleteByUserID(ctx, userID)
	}
	return nil
}

// --- Mock Mailer ---

type mockMailer struct {
	sendVerificationCode func(to, code string) error

	// Счётчики вызовов для проверки
	sendCalled bool
	lastTo     string
	lastCode   string
}

func (m *mockMailer) SendVerificationCode(to, code string) error {
	m.sendCalled = true
	m.lastTo = to
	m.lastCode = code
	if m.sendVerificationCode != nil {
		return m.sendVerificationCode(to, code)
	}
	return nil
}

// --- Helper ---

func hashPassword(t *testing.T, password string) string {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 4)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	return string(hash)
}

// =============================================================================
// TESTS
// =============================================================================

func TestRegister_InvalidEmail(t *testing.T) {
	userRepo := &mockUserRepo{}
	verificationRepo := &mockVerificationRepo{}
	sessionRepo := &mockSessionRepo{}
	mailer := &mockMailer{}
	cfg := testAuthConfig()

	svc := NewAuthService(userRepo, verificationRepo, sessionRepo, mailer, cfg)

	ctx := context.Background()
	_, err := svc.Register(ctx, "bad-email", "password123")

	if err != ErrInvalidEmail {
		t.Errorf("expected ErrInvalidEmail, got %v", err)
	}

	// Проверяем, что mailer не вызывался
	if mailer.sendCalled {
		t.Error("mailer should not be called for invalid email")
	}
}

func TestRegister_ShortPassword(t *testing.T) {
	userRepo := &mockUserRepo{}
	verificationRepo := &mockVerificationRepo{}
	sessionRepo := &mockSessionRepo{}
	mailer := &mockMailer{}
	cfg := testAuthConfig()

	svc := NewAuthService(userRepo, verificationRepo, sessionRepo, mailer, cfg)

	ctx := context.Background()
	_, err := svc.Register(ctx, "test@example.com", "123")

	if err != ErrPasswordTooShort {
		t.Errorf("expected ErrPasswordTooShort, got %v", err)
	}

	// Проверяем, что mailer не вызывался
	if mailer.sendCalled {
		t.Error("mailer should not be called for short password")
	}
}

func TestLogin_EmailNotVerified(t *testing.T) {
	passwordHash := hashPassword(t, "password123")

	userRepo := &mockUserRepo{
		getByEmail: func(ctx context.Context, email string) (*models.User, error) {
			return &models.User{
				ID:            1,
				Email:         email,
				PasswordHash:  passwordHash,
				EmailVerified: false,
			}, nil
		},
	}
	verificationRepo := &mockVerificationRepo{}
	sessionRepo := &mockSessionRepo{}
	mailer := &mockMailer{}
	cfg := testAuthConfig()

	svc := NewAuthService(userRepo, verificationRepo, sessionRepo, mailer, cfg)

	ctx := context.Background()
	_, err := svc.Login(ctx, "test@example.com", "password123")

	if err != ErrEmailNotVerified {
		t.Errorf("expected ErrEmailNotVerified, got %v", err)
	}

	// Проверяем, что сессия не создавалась
	if sessionRepo.createCalled {
		t.Error("session should not be created for unverified email")
	}
}

func TestConfirmEmail_WrongCode_IncrementsAttempts(t *testing.T) {
	correctCode := "111111"
	wrongCode := "222222"
	codeHash := hashPassword(t, correctCode)

	userRepo := &mockUserRepo{
		getByEmail: func(ctx context.Context, email string) (*models.User, error) {
			return &models.User{
				ID:            1,
				Email:         email,
				EmailVerified: false,
			}, nil
		},
	}

	setEmailVerifiedCalled := false
	userRepo.setEmailVerified = func(ctx context.Context, userID int, verified bool) error {
		setEmailVerifiedCalled = true
		return nil
	}

	verificationRepo := &mockVerificationRepo{
		getByUserID: func(ctx context.Context, userID int) (*models.EmailVerificationCode, error) {
			return &models.EmailVerificationCode{
				ID:        1,
				UserID:    userID,
				CodeHash:  codeHash,
				Attempts:  0,
				ExpiresAt: time.Now().Add(10 * time.Minute),
			}, nil
		},
	}

	sessionRepo := &mockSessionRepo{}
	mailer := &mockMailer{}
	cfg := testAuthConfig()

	svc := NewAuthService(userRepo, verificationRepo, sessionRepo, mailer, cfg)

	ctx := context.Background()
	_, err := svc.ConfirmEmail(ctx, "test@example.com", wrongCode)

	if err != ErrInvalidCode {
		t.Errorf("expected ErrInvalidCode, got %v", err)
	}

	// Проверяем, что IncrementAttempts был вызван
	if !verificationRepo.incrementAttemptsCalled {
		t.Error("IncrementAttempts should be called for wrong code")
	}

	// Проверяем, что SetEmailVerified НЕ вызывался
	if setEmailVerifiedCalled {
		t.Error("SetEmailVerified should not be called for wrong code")
	}

	// Проверяем, что сессия не создавалась
	if sessionRepo.createCalled {
		t.Error("session should not be created for wrong code")
	}
}

func TestConfirmEmail_TooManyAttempts(t *testing.T) {
	correctCode := "111111"
	codeHash := hashPassword(t, correctCode)
	cfg := testAuthConfig()

	userRepo := &mockUserRepo{
		getByEmail: func(ctx context.Context, email string) (*models.User, error) {
			return &models.User{
				ID:            1,
				Email:         email,
				EmailVerified: false,
			}, nil
		},
	}

	verificationRepo := &mockVerificationRepo{
		getByUserID: func(ctx context.Context, userID int) (*models.EmailVerificationCode, error) {
			return &models.EmailVerificationCode{
				ID:        1,
				UserID:    userID,
				CodeHash:  codeHash,
				Attempts:  cfg.MaxVerifyAttempts, // Уже достигли лимита
				ExpiresAt: time.Now().Add(10 * time.Minute),
			}, nil
		},
	}

	sessionRepo := &mockSessionRepo{}
	mailer := &mockMailer{}

	svc := NewAuthService(userRepo, verificationRepo, sessionRepo, mailer, cfg)

	ctx := context.Background()
	_, err := svc.ConfirmEmail(ctx, "test@example.com", correctCode)

	if err != ErrTooManyAttempts {
		t.Errorf("expected ErrTooManyAttempts, got %v", err)
	}

	// Проверяем, что сессия не создавалась
	if sessionRepo.createCalled {
		t.Error("session should not be created when too many attempts")
	}

	// IncrementAttempts не должен вызываться - мы отклоняем ДО проверки кода
	if verificationRepo.incrementAttemptsCalled {
		t.Error("IncrementAttempts should not be called when already at max attempts")
	}
}

func TestConfirmEmail_ExpiredCode(t *testing.T) {
	correctCode := "111111"
	codeHash := hashPassword(t, correctCode)

	userRepo := &mockUserRepo{
		getByEmail: func(ctx context.Context, email string) (*models.User, error) {
			return &models.User{
				ID:            1,
				Email:         email,
				EmailVerified: false,
			}, nil
		},
	}

	verificationRepo := &mockVerificationRepo{
		getByUserID: func(ctx context.Context, userID int) (*models.EmailVerificationCode, error) {
			return &models.EmailVerificationCode{
				ID:        1,
				UserID:    userID,
				CodeHash:  codeHash,
				Attempts:  0,
				ExpiresAt: time.Now().Add(-10 * time.Minute), // Истёк
			}, nil
		},
	}

	sessionRepo := &mockSessionRepo{}
	mailer := &mockMailer{}
	cfg := testAuthConfig()

	svc := NewAuthService(userRepo, verificationRepo, sessionRepo, mailer, cfg)

	ctx := context.Background()
	_, err := svc.ConfirmEmail(ctx, "test@example.com", correctCode)

	if err != ErrInvalidCode {
		t.Errorf("expected ErrInvalidCode for expired code, got %v", err)
	}

	if sessionRepo.createCalled {
		t.Error("session should not be created for expired code")
	}
}

func TestLogin_InvalidCredentials_WrongPassword(t *testing.T) {
	passwordHash := hashPassword(t, "correctpassword")

	userRepo := &mockUserRepo{
		getByEmail: func(ctx context.Context, email string) (*models.User, error) {
			return &models.User{
				ID:            1,
				Email:         email,
				PasswordHash:  passwordHash,
				EmailVerified: true,
			}, nil
		},
	}

	verificationRepo := &mockVerificationRepo{}
	sessionRepo := &mockSessionRepo{}
	mailer := &mockMailer{}
	cfg := testAuthConfig()

	svc := NewAuthService(userRepo, verificationRepo, sessionRepo, mailer, cfg)

	ctx := context.Background()
	_, err := svc.Login(ctx, "test@example.com", "wrongpassword")

	if err != ErrInvalidCredentials {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}

	if sessionRepo.createCalled {
		t.Error("session should not be created for wrong password")
	}
}

func TestLogin_Success(t *testing.T) {
	passwordHash := hashPassword(t, "password123")

	userRepo := &mockUserRepo{
		getByEmail: func(ctx context.Context, email string) (*models.User, error) {
			return &models.User{
				ID:            1,
				Email:         email,
				PasswordHash:  passwordHash,
				EmailVerified: true,
			}, nil
		},
	}

	verificationRepo := &mockVerificationRepo{}
	sessionRepo := &mockSessionRepo{}
	mailer := &mockMailer{}
	cfg := testAuthConfig()

	svc := NewAuthService(userRepo, verificationRepo, sessionRepo, mailer, cfg)

	ctx := context.Background()
	result, err := svc.Login(ctx, "test@example.com", "password123")

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if result == nil {
		t.Fatal("expected result, got nil")
	}

	if result.UserID != 1 {
		t.Errorf("expected user_id=1, got %d", result.UserID)
	}

	if result.SessionToken == "" {
		t.Error("expected session token, got empty string")
	}

	if !sessionRepo.createCalled {
		t.Error("session should be created on successful login")
	}
}

func TestConfirmEmail_Success(t *testing.T) {
	correctCode := "111111"
	codeHash := hashPassword(t, correctCode)

	setEmailVerifiedCalled := false
	deleteVerificationCalled := false

	userRepo := &mockUserRepo{
		getByEmail: func(ctx context.Context, email string) (*models.User, error) {
			return &models.User{
				ID:            1,
				Email:         email,
				EmailVerified: false,
			}, nil
		},
		setEmailVerified: func(ctx context.Context, userID int, verified bool) error {
			setEmailVerifiedCalled = true
			if userID != 1 || !verified {
				t.Errorf("unexpected setEmailVerified call: userID=%d, verified=%v", userID, verified)
			}
			return nil
		},
	}

	verificationRepo := &mockVerificationRepo{
		getByUserID: func(ctx context.Context, userID int) (*models.EmailVerificationCode, error) {
			return &models.EmailVerificationCode{
				ID:        1,
				UserID:    userID,
				CodeHash:  codeHash,
				Attempts:  0,
				ExpiresAt: time.Now().Add(10 * time.Minute),
			}, nil
		},
		delete: func(ctx context.Context, id int) error {
			deleteVerificationCalled = true
			return nil
		},
	}

	sessionRepo := &mockSessionRepo{}
	mailer := &mockMailer{}
	cfg := testAuthConfig()

	svc := NewAuthService(userRepo, verificationRepo, sessionRepo, mailer, cfg)

	ctx := context.Background()
	result, err := svc.ConfirmEmail(ctx, "test@example.com", correctCode)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if result == nil {
		t.Fatal("expected result, got nil")
	}

	if result.UserID != 1 {
		t.Errorf("expected user_id=1, got %d", result.UserID)
	}

	if result.SessionToken == "" {
		t.Error("expected session token, got empty string")
	}

	if !setEmailVerifiedCalled {
		t.Error("SetEmailVerified should be called on successful confirm")
	}

	if !deleteVerificationCalled {
		t.Error("Delete verification should be called on successful confirm")
	}

	if !sessionRepo.createCalled {
		t.Error("session should be created on successful confirm")
	}
}

func TestRegister_NewUser_Success(t *testing.T) {
	createCalled := false
	createdPasswordHash := ""

	userRepo := &mockUserRepo{
		getByEmail: func(ctx context.Context, email string) (*models.User, error) {
			return nil, sql.ErrNoRows // Пользователь не существует
		},
		getByUsername: func(ctx context.Context, username string) (*models.User, error) {
			return nil, sql.ErrNoRows // Username свободен
		},
		create: func(ctx context.Context, username, email, passwordHash string) (*models.User, error) {
			createCalled = true
			createdPasswordHash = passwordHash
			return &models.User{
				ID:       1,
				Username: username,
				Email:    email,
			}, nil
		},
	}

	verificationRepo := &mockVerificationRepo{}
	sessionRepo := &mockSessionRepo{}
	mailer := &mockMailer{}
	cfg := testAuthConfig()

	svc := NewAuthService(userRepo, verificationRepo, sessionRepo, mailer, cfg)

	ctx := context.Background()
	result, err := svc.Register(ctx, "new@example.com", "password123")

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if result == nil {
		t.Fatal("expected result, got nil")
	}

	if !result.IsNew {
		t.Error("expected IsNew=true for new user")
	}

	if !createCalled {
		t.Error("Create should be called for new user")
	}

	// проверяем что пароль был захеширован
	if createdPasswordHash == "password123" {
		t.Error("password should be hashed, not stored as plain text")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(createdPasswordHash), []byte("password123")); err != nil {
		t.Error("stored password hash should match original password")
	}

	if !mailer.sendCalled {
		t.Error("verification email should be sent")
	}

	if mailer.lastTo != "new@example.com" {
		t.Errorf("email should be sent to new@example.com, got %s", mailer.lastTo)
	}
}