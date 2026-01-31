package database

import (
	"context"
	"database/sql"
	"time"

	"bytebattle/internal/database/models"

	"github.com/aarondl/sqlboiler/v4/boil"
	"github.com/aarondl/null/v8"
)

type SessionRepository interface {
	Create(ctx context.Context, userID int, tokenHash string, expiresAt time.Time) (*models.AuthSession, error)
	GetByTokenHash(ctx context.Context, tokenHash string) (*models.AuthSession, error)
	Revoke(ctx context.Context, tokenHash string) error
	DeleteExpired(ctx context.Context) (int64, error)
	DeleteByUserID(ctx context.Context, userID int) error
}

type sessionRepository struct {
	db *sql.DB
}

func NewSessionRepository(db *sql.DB) SessionRepository {
	return &sessionRepository{db: db}
}

func (r *sessionRepository) Create(ctx context.Context, userID int, tokenHash string, expiresAt time.Time) (*models.AuthSession, error) {
	session := &models.AuthSession{
		UserID:    userID,
		TokenHash: tokenHash,
		ExpiresAt: expiresAt,
	}

	if err := session.Insert(ctx, r.db, boil.Infer()); err != nil {
		return nil, err
	}

	return session, nil
}

func (r *sessionRepository) GetByTokenHash(ctx context.Context, tokenHash string) (*models.AuthSession, error) {
	return models.AuthSessions(
		models.AuthSessionWhere.TokenHash.EQ(tokenHash),
		models.AuthSessionWhere.RevokedAt.IsNull(),
		models.AuthSessionWhere.ExpiresAt.GT(time.Now()),
	).One(ctx, r.db)
}

func (r *sessionRepository) Revoke(ctx context.Context, tokenHash string) error {
	now := null.TimeFrom(time.Now())
	_, err := models.AuthSessions(
		models.AuthSessionWhere.TokenHash.EQ(tokenHash),
	).UpdateAll(ctx, r.db, models.M{
		models.AuthSessionColumns.RevokedAt: now,
	})
	return err
}

func (r *sessionRepository) DeleteExpired(ctx context.Context) (int64, error) {
	return models.AuthSessions(
		models.AuthSessionWhere.ExpiresAt.LT(time.Now()),
	).DeleteAll(ctx, r.db)
}

func (r *sessionRepository) DeleteByUserID(ctx context.Context, userID int) error {
	_, err := models.AuthSessions(
		models.AuthSessionWhere.UserID.EQ(userID),
	).DeleteAll(ctx, r.db)
	return err
}