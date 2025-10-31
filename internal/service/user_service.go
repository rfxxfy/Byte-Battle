package service

import (
	"context"
	"database/sql"
	"errors"

	"bytebattle/internal/database"
	"bytebattle/internal/database/models"
)

type UserService struct {
	repo database.UserRepository
}

func NewUserService(repo database.UserRepository) *UserService {
	return &UserService{repo: repo}
}

func (s *UserService) GetOrCreateTestUser(ctx context.Context) (*models.User, error) {
	user, err := s.repo.GetByUsername(ctx, "testuser")
	if errors.Is(err, sql.ErrNoRows) {
		return s.repo.Create(ctx, "testuser", "test@example.com", "hashedpassword")
	}
	return user, err
}
