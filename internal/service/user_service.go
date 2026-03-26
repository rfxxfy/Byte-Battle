package service

import (
	"context"
	"errors"
	"strings"

	sqlcdb "bytebattle/internal/db/sqlc"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var ErrInvalidName = errors.New("name must be between 1 and 100 characters")

type UserService struct {
	q *sqlcdb.Queries
}

func NewUserService(q *sqlcdb.Queries) *UserService {
	return &UserService{q: q}
}

func (s *UserService) GetByID(ctx context.Context, id uuid.UUID) (sqlcdb.User, error) {
	return s.q.GetUserByID(ctx, id)
}

func (s *UserService) UpdateName(ctx context.Context, id uuid.UUID, name string) (sqlcdb.User, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" || len(trimmed) > 100 {
		return sqlcdb.User{}, ErrInvalidName
	}
	return s.q.UpdateUserName(ctx, sqlcdb.UpdateUserNameParams{
		ID:   id,
		Name: pgtype.Text{String: trimmed, Valid: true},
	})
}

func (s *UserService) GetOrCreateTestUser(ctx context.Context) (sqlcdb.User, error) {
	user, err := s.q.GetUserByUsername(ctx, "testuser")
	if errors.Is(err, pgx.ErrNoRows) {
		return s.q.CreateUser(ctx, sqlcdb.CreateUserParams{
			Username:     "testuser",
			Email:        "test@example.com",
			PasswordHash: pgtype.Text{String: "hashedpassword", Valid: true},
		})
	}
	return user, err
}
