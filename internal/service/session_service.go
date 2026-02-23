package service

import (
	"context"
	"errors"
	"time"

	"bytebattle/internal/database"
	"bytebattle/internal/database/models"
)

var (
	ErrSessionNotFound = errors.New("session not found")
	ErrSessionExpired  = errors.New("session expired")
	ErrInvalidToken    = errors.New("invalid token")
)

const DefaultSessionDuration = 24 * time.Hour

type SessionService struct {
	repo            database.ISessionRepo
	sessionDuration time.Duration
}

type SessionServiceOption func(*SessionService)

func WithSessionDuration(d time.Duration) SessionServiceOption {
	return func(s *SessionService) {
		s.sessionDuration = d
	}
}

func NewSessionService(repo database.ISessionRepo, opts ...SessionServiceOption) *SessionService {
	s := &SessionService{
		repo:            repo,
		sessionDuration: DefaultSessionDuration,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

func (s *SessionService) CreateSession(ctx context.Context, userID int) (*models.Session, error) {
	expiresAt := time.Now().Add(s.sessionDuration)
	return s.repo.Create(ctx, userID, expiresAt)
}

func (s *SessionService) CreateSessionWithDuration(ctx context.Context, userID int, duration time.Duration) (*models.Session, error) {
	expiresAt := time.Now().Add(duration)
	return s.repo.Create(ctx, userID, expiresAt)
}

func (s *SessionService) GetSession(ctx context.Context, id int) (*models.Session, error) {
	session, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return nil, ErrSessionNotFound
		}
		return nil, err
	}

	return session, nil
}

func (s *SessionService) ValidateToken(ctx context.Context, token string) (*models.Session, error) {
	if token == "" {
		return nil, ErrInvalidToken
	}

	session, err := s.repo.GetByToken(ctx, token)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return nil, ErrSessionNotFound
		}
		return nil, err
	}

	if time.Now().After(session.ExpiresAt) {
		return nil, ErrSessionExpired
	}

	return session, nil
}

func (s *SessionService) GetUserSessions(ctx context.Context, userID int) (models.SessionSlice, error) {
	return s.repo.GetByUserID(ctx, userID)
}

func (s *SessionService) RefreshSession(ctx context.Context, id int) (*models.Session, error) {
	session, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return nil, ErrSessionNotFound
		}
		return nil, err
	}

	session.ExpiresAt = time.Now().Add(s.sessionDuration)

	if err := s.repo.Update(ctx, session); err != nil {
		return nil, err
	}

	return session, nil
}

func (s *SessionService) RefreshSessionByToken(ctx context.Context, token string) (*models.Session, error) {
	session, err := s.repo.GetByToken(ctx, token)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return nil, ErrSessionNotFound
		}
		return nil, err
	}

	session.ExpiresAt = time.Now().Add(s.sessionDuration)

	if err := s.repo.Update(ctx, session); err != nil {
		return nil, err
	}

	return session, nil
}

func (s *SessionService) EndSession(ctx context.Context, id int) error {
	err := s.repo.Delete(ctx, id)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return ErrSessionNotFound
		}
		return err
	}
	return nil
}

func (s *SessionService) EndSessionByToken(ctx context.Context, token string) error {
	err := s.repo.DeleteByToken(ctx, token)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return ErrSessionNotFound
		}
		return err
	}
	return nil
}

func (s *SessionService) EndAllUserSessions(ctx context.Context, userID int) (int64, error) {
	return s.repo.DeleteByUserID(ctx, userID)
}

func (s *SessionService) CleanupExpired(ctx context.Context) (int64, error) {
	return s.repo.DeleteExpired(ctx)
}
