package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"bytebattle/internal/apierr"
	sqlcdb "bytebattle/internal/db/sqlc"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const DefaultSessionDuration = 24 * time.Hour

type SessionService struct {
	q               *sqlcdb.Queries
	sessionDuration time.Duration
}

type SessionServiceOption func(*SessionService)

func WithSessionDuration(d time.Duration) SessionServiceOption {
	return func(s *SessionService) {
		s.sessionDuration = d
	}
}

func NewSessionService(q *sqlcdb.Queries, opts ...SessionServiceOption) *SessionService {
	s := &SessionService{
		q:               q,
		sessionDuration: DefaultSessionDuration,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

func generateToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func (s *SessionService) CreateSession(ctx context.Context, userID int) (sqlcdb.Session, error) {
	return s.createSession(ctx, userID, s.sessionDuration)
}

func (s *SessionService) CreateSessionWithDuration(ctx context.Context, userID int, duration time.Duration) (sqlcdb.Session, error) {
	return s.createSession(ctx, userID, duration)
}

func (s *SessionService) createSession(ctx context.Context, userID int, duration time.Duration) (sqlcdb.Session, error) {
	token, err := generateToken()
	if err != nil {
		return sqlcdb.Session{}, err
	}

	expiresAt := time.Now().Add(duration)

	return s.q.CreateSession(ctx, sqlcdb.CreateSessionParams{
		UserID:    int32(userID),
		Token:     token,
		ExpiresAt: pgtype.Timestamptz{Time: expiresAt.UTC(), Valid: true},
	})
}

func (s *SessionService) GetSession(ctx context.Context, id int) (sqlcdb.Session, error) {
	session, err := s.q.GetSessionByID(ctx, int32(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlcdb.Session{}, apierr.New(apierr.ErrSessionNotFound, "session not found")
	}
	return session, err
}

func (s *SessionService) ValidateToken(ctx context.Context, token string) (sqlcdb.Session, error) {
	if token == "" {
		return sqlcdb.Session{}, apierr.New(apierr.ErrInvalidToken, "token is required")
	}

	session, err := s.q.GetSessionByToken(ctx, token)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlcdb.Session{}, apierr.New(apierr.ErrSessionNotFound, "session not found")
	}
	if err != nil {
		return sqlcdb.Session{}, err
	}

	if time.Now().After(session.ExpiresAt.Time) {
		return sqlcdb.Session{}, apierr.New(apierr.ErrSessionExpired, "session expired")
	}

	return session, nil
}

func (s *SessionService) GetUserSessions(ctx context.Context, userID int) ([]sqlcdb.Session, error) {
	return s.q.GetSessionsByUserID(ctx, int32(userID))
}

func (s *SessionService) RefreshSession(ctx context.Context, id int) (sqlcdb.Session, error) {
	newExpiry := time.Now().Add(s.sessionDuration)

	session, err := s.q.UpdateSessionExpiry(ctx, sqlcdb.UpdateSessionExpiryParams{
		ID:        int32(id),
		ExpiresAt: pgtype.Timestamptz{Time: newExpiry.UTC(), Valid: true},
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlcdb.Session{}, apierr.New(apierr.ErrSessionNotFound, "session not found")
	}
	return session, err
}

func (s *SessionService) EndSession(ctx context.Context, id int) error {
	n, err := s.q.DeleteSession(ctx, int32(id))
	if err != nil {
		return err
	}
	if n == 0 {
		return apierr.New(apierr.ErrSessionNotFound, "session not found")
	}
	return nil
}

func (s *SessionService) EndAllUserSessions(ctx context.Context, userID int) (int64, error) {
	return s.q.DeleteSessionsByUserID(ctx, int32(userID))
}

func (s *SessionService) CleanupExpired(ctx context.Context) (int64, error) {
	return s.q.DeleteExpiredSessions(ctx)
}
