package database

import (
	"context"
	"database/sql"

	"bytebattle/internal/database/models"

	"github.com/aarondl/sqlboiler/v4/boil"
)

type UserRepository interface {
	GetByID(ctx context.Context, id int) (*models.User, error)
	GetByUsername(ctx context.Context, username string) (*models.User, error)
	GetByEmail(ctx context.Context, email string) (*models.User, error)
	Create(ctx context.Context, username, email, passwordHash string) (*models.User, error)
	SetEmailVerified(ctx context.Context, userID int, verified bool) error
	UpdatePasswordHash(ctx context.Context, userID int, passwordHash string) error
}

type userRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) GetByID(ctx context.Context, id int) (*models.User, error) {
	return models.FindUser(ctx, r.db, id)
}

func (r *userRepository) GetByUsername(ctx context.Context, username string) (*models.User, error) {
	return models.Users(
		models.UserWhere.Username.EQ(username),
	).One(ctx, r.db)
}

func (r *userRepository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	return models.Users(
		models.UserWhere.Email.EQ(email),
	).One(ctx, r.db)
}

func (r *userRepository) Create(ctx context.Context, username, email, passwordHash string) (*models.User, error) {
	u := &models.User{
		Username:     username,
		Email:        email,
		PasswordHash: passwordHash,
	}

	if err := u.Insert(ctx, r.db, boil.Infer()); err != nil {
		return nil, err
	}

	return u, nil
}

func (r *userRepository) SetEmailVerified(ctx context.Context, userID int, verified bool) error {
	user, err := models.FindUser(ctx, r.db, userID)
	if err != nil {
		return err
	}
	user.EmailVerified = verified
	_, err = user.Update(ctx, r.db, boil.Whitelist(models.UserColumns.EmailVerified))
	return err
}

func (r *userRepository) UpdatePasswordHash(ctx context.Context, userID int, passwordHash string) error {
	user, err := models.FindUser(ctx, r.db, userID)
	if err != nil {
		return err
	}
	user.PasswordHash = passwordHash
	_, err = user.Update(ctx, r.db, boil.Whitelist(models.UserColumns.PasswordHash))
	return err
}