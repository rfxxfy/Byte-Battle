package database

import (
	"context"
	"database/sql"

	"github.com/aarondl/sqlboiler/v4/boil"

	"bytebattle/internal/database/models"
)

type UserRepository interface {
	GetByUsername(ctx context.Context, username string) (*models.User, error)
	Create(ctx context.Context, username, email, password string) (*models.User, error)
}

type userRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) GetByUsername(
	ctx context.Context,
	username string,
) (*models.User, error) {
	return models.Users(
		models.UserWhere.Username.EQ(username),
	).One(ctx, r.db)
}

func (r *userRepository) Create(
	ctx context.Context,
	username, email, password string,
) (*models.User, error) {
	u := &models.User{
		Username:     username,
		Email:        email,
		PasswordHash: password,
	}

	if err := u.Insert(ctx, r.db, boil.Infer()); err != nil {
		return nil, err
	}

	return u, nil
}
