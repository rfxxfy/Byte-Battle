package database

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"time"

	"bytebattle/internal/database/models"

	"github.com/aarondl/sqlboiler/v4/boil"
	"github.com/aarondl/sqlboiler/v4/queries/qm"
)

type ISessionRepo interface {
	Create(ctx context.Context, userID int, expiresAt time.Time) (*models.Session, error)
	GetByID(ctx context.Context, id int) (*models.Session, error)
	GetByToken(ctx context.Context, token string) (*models.Session, error)
	GetByUserID(ctx context.Context, userID int) (models.SessionSlice, error)
	Update(ctx context.Context, session *models.Session) error
	Delete(ctx context.Context, id int) error
	DeleteByToken(ctx context.Context, token string) error
	DeleteExpired(ctx context.Context) (int64, error)
	DeleteByUserID(ctx context.Context, userID int) (int64, error)
}

type sessionRepo struct {
	db *sql.DB
}

func NewSessionRepository(db *sql.DB) ISessionRepo {
	return &sessionRepo{db: db}
}

func generateToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func (r *sessionRepo) Create(ctx context.Context, userID int, expiresAt time.Time) (*models.Session, error) {
	token, err := generateToken()
	if err != nil {
		return nil, err
	}

	session := &models.Session{
		UserID:    userID,
		Token:     token,
		ExpiresAt: expiresAt,
	}

	if err := session.Insert(ctx, r.db, boil.Infer()); err != nil {
		return nil, err
	}

	return session, nil
}

func (r *sessionRepo) GetByID(ctx context.Context, id int) (*models.Session, error) {
	return models.FindSession(ctx, r.db, id)
}

func (r *sessionRepo) GetByToken(ctx context.Context, token string) (*models.Session, error) {
	return models.Sessions(
		models.SessionWhere.Token.EQ(token),
	).One(ctx, r.db)
}

func (r *sessionRepo) GetByUserID(ctx context.Context, userID int) (models.SessionSlice, error) {
	return models.Sessions(
		models.SessionWhere.UserID.EQ(userID),
		qm.OrderBy("created_at DESC"),
	).All(ctx, r.db)
}

func (r *sessionRepo) Update(ctx context.Context, session *models.Session) error {
	session.UpdatedAt = time.Now()
	_, err := session.Update(ctx, r.db, boil.Infer())
	return err
}

func (r *sessionRepo) Delete(ctx context.Context, id int) error {
	session, err := models.FindSession(ctx, r.db, id)
	if err != nil {
		return err
	}

	_, err = session.Delete(ctx, r.db)
	return err
}

func (r *sessionRepo) DeleteByToken(ctx context.Context, token string) error {
	session, err := r.GetByToken(ctx, token)
	if err != nil {
		return err
	}

	_, err = session.Delete(ctx, r.db)
	return err
}

func (r *sessionRepo) DeleteExpired(ctx context.Context) (int64, error) {
	return models.Sessions(
		models.SessionWhere.ExpiresAt.LT(time.Now()),
	).DeleteAll(ctx, r.db)
}

func (r *sessionRepo) DeleteByUserID(ctx context.Context, userID int) (int64, error) {
	return models.Sessions(
		models.SessionWhere.UserID.EQ(userID),
	).DeleteAll(ctx, r.db)
}
