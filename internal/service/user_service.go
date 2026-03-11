package service

import (
	"context"
	"errors"

	sqlcdb "bytebattle/internal/db/sqlc"

	"github.com/jackc/pgx/v5"
)

type UserService struct {
	q *sqlcdb.Queries
}

func NewUserService(q *sqlcdb.Queries) *UserService {
	return &UserService{q: q}
}

func (s *UserService) GetOrCreateTestUser(ctx context.Context) (sqlcdb.User, error) {
	user, err := s.q.GetUserByUsername(ctx, "testuser")
	if errors.Is(err, pgx.ErrNoRows) {
		return s.q.CreateUser(ctx, sqlcdb.CreateUserParams{
			Username:     "testuser",
			Email:        "test@example.com",
			PasswordHash: "hashedpassword",
		})
	}
	return user, err
}
