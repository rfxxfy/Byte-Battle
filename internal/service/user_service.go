package service

import (
	"context"
	"errors"

	"bytebattle/internal/database"
	"bytebattle/internal/database/models"
)

type UserService struct {
	repo database.IUserRepo
}

func NewUserService(repo database.IUserRepo) *UserService {
	return &UserService{repo: repo}
}

func (s *UserService) GetOrCreateTestUser(ctx context.Context) (*models.User, error) {
	user, err := s.repo.GetByUsername(ctx, "testuser")
	if errors.Is(err, database.ErrNotFound) {
		return s.repo.Create(ctx, "testuser", "test@example.com", "hashedpassword")
	}
	return user, err
}
