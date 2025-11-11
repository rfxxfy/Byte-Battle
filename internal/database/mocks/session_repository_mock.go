package mocks

import (
	"context"
	"time"

	"bytebattle/internal/database/models"

	"github.com/stretchr/testify/mock"
)

type MockSessionRepo struct {
	mock.Mock
}

func NewMockSessionRepo() *MockSessionRepo {
	return &MockSessionRepo{}
}

func (m *MockSessionRepo) Create(ctx context.Context, userID int, expiresAt time.Time) (*models.Session, error) {
	args := m.Called(ctx, userID, expiresAt)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Session), args.Error(1)
}

func (m *MockSessionRepo) GetByID(ctx context.Context, id int) (*models.Session, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Session), args.Error(1)
}

func (m *MockSessionRepo) GetByToken(ctx context.Context, token string) (*models.Session, error) {
	args := m.Called(ctx, token)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Session), args.Error(1)
}

func (m *MockSessionRepo) GetByUserID(ctx context.Context, userID int) (models.SessionSlice, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(models.SessionSlice), args.Error(1)
}

func (m *MockSessionRepo) Update(ctx context.Context, session *models.Session) error {
	args := m.Called(ctx, session)
	return args.Error(0)
}

func (m *MockSessionRepo) Delete(ctx context.Context, id int) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockSessionRepo) DeleteByToken(ctx context.Context, token string) error {
	args := m.Called(ctx, token)
	return args.Error(0)
}

func (m *MockSessionRepo) DeleteExpired(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockSessionRepo) DeleteByUserID(ctx context.Context, userID int) (int64, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).(int64), args.Error(1)
}
