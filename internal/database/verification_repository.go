package database

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"bytebattle/internal/database/models"

	"github.com/aarondl/sqlboiler/v4/boil"
)

type VerificationRepository interface {
	Upsert(ctx context.Context, userID int, codeHash string, expiresAt time.Time) (*models.EmailVerificationCode, error)
	GetByUserID(ctx context.Context, userID int) (*models.EmailVerificationCode, error)
	IncrementAttempts(ctx context.Context, id int) error
	Delete(ctx context.Context, id int) error
	DeleteByUserID(ctx context.Context, userID int) error
}

type verificationRepository struct {
	db *sql.DB
}

func NewVerificationRepository(db *sql.DB) VerificationRepository {
	return &verificationRepository{db: db}
}

func (r *verificationRepository) Upsert(ctx context.Context, userID int, codeHash string, expiresAt time.Time) (*models.EmailVerificationCode, error) {
	existing, err := models.EmailVerificationCodes(
		models.EmailVerificationCodeWhere.UserID.EQ(userID),
	).One(ctx, r.db)

	if errors.Is(err, sql.ErrNoRows) {
		code := &models.EmailVerificationCode{
			UserID:    userID,
			CodeHash:  codeHash,
			ExpiresAt: expiresAt,
			Attempts:  0,
		}
		if err := code.Insert(ctx, r.db, boil.Infer()); err != nil {
			return nil, err
		}
		return code, nil
	}

	if err != nil {
		return nil, err
	}

	existing.CodeHash = codeHash
	existing.ExpiresAt = expiresAt
	existing.Attempts = 0

	_, err = existing.Update(ctx, r.db, boil.Whitelist(
		models.EmailVerificationCodeColumns.CodeHash,
		models.EmailVerificationCodeColumns.ExpiresAt,
		models.EmailVerificationCodeColumns.Attempts,
	))
	if err != nil {
		return nil, err
	}

	return existing, nil
}

func (r *verificationRepository) GetByUserID(ctx context.Context, userID int) (*models.EmailVerificationCode, error) {
	return models.EmailVerificationCodes(
		models.EmailVerificationCodeWhere.UserID.EQ(userID),
	).One(ctx, r.db)
}

func (r *verificationRepository) IncrementAttempts(ctx context.Context, id int) error {
	code, err := models.FindEmailVerificationCode(ctx, r.db, id)
	if err != nil {
		return err
	}
	code.Attempts = code.Attempts + 1
	_, err = code.Update(ctx, r.db, boil.Whitelist(models.EmailVerificationCodeColumns.Attempts))
	return err
}

func (r *verificationRepository) Delete(ctx context.Context, id int) error {
	code, err := models.FindEmailVerificationCode(ctx, r.db, id)
	if err != nil {
		return err
	}
	_, err = code.Delete(ctx, r.db)
	return err
}

func (r *verificationRepository) DeleteByUserID(ctx context.Context, userID int) error {
	_, err := models.EmailVerificationCodes(
		models.EmailVerificationCodeWhere.UserID.EQ(userID),
	).DeleteAll(ctx, r.db)
	return err
}