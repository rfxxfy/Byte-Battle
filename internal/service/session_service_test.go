package service

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"bytebattle/internal/database"
	"bytebattle/internal/database/mocks"
	"bytebattle/internal/database/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type SessionServiceTestSuite struct {
	suite.Suite
	mockRepo *mocks.MockSessionRepo
	service  *SessionService
	ctx      context.Context
}

func (s *SessionServiceTestSuite) SetupTest() {
	s.mockRepo = mocks.NewMockSessionRepo()
	s.service = NewSessionService(s.mockRepo, WithSessionDuration(time.Hour))
	s.ctx = context.Background()
}

func (s *SessionServiceTestSuite) TearDownTest() {
	s.mockRepo.AssertExpectations(s.T())
}

func TestSessionServiceTestSuite(t *testing.T) {
	suite.Run(t, new(SessionServiceTestSuite))
}

// CreateSession tests

func (s *SessionServiceTestSuite) TestCreateSession_Success() {
	userID := 1
	expectedSession := &models.Session{
		ID:        1,
		UserID:    userID,
		Token:     "test-token",
		ExpiresAt: time.Now().Add(time.Hour),
	}

	s.mockRepo.On("Create", s.ctx, userID, mock.AnythingOfType("time.Time")).
		Return(expectedSession, nil)

	session, err := s.service.CreateSession(s.ctx, userID)

	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), session)
	assert.Equal(s.T(), userID, session.UserID)
}

func (s *SessionServiceTestSuite) TestCreateSession_RepoError() {
	userID := 1
	expectedErr := sql.ErrConnDone

	s.mockRepo.On("Create", s.ctx, userID, mock.AnythingOfType("time.Time")).
		Return(nil, expectedErr)

	session, err := s.service.CreateSession(s.ctx, userID)

	assert.Error(s.T(), err)
	assert.Nil(s.T(), session)
	assert.Equal(s.T(), expectedErr, err)
}

// CreateSessionWithDuration tests

func (s *SessionServiceTestSuite) TestCreateSessionWithDuration_Success() {
	userID := 1
	duration := 2 * time.Hour
	expectedSession := &models.Session{
		ID:        1,
		UserID:    userID,
		Token:     "test-token",
		ExpiresAt: time.Now().Add(duration),
	}

	s.mockRepo.On("Create", s.ctx, userID, mock.AnythingOfType("time.Time")).
		Return(expectedSession, nil)

	session, err := s.service.CreateSessionWithDuration(s.ctx, userID, duration)

	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), session)
}

// GetSession tests

func (s *SessionServiceTestSuite) TestGetSession_Success() {
	sessionID := 1
	expectedSession := &models.Session{
		ID:     sessionID,
		UserID: 1,
		Token:  "test-token",
	}

	s.mockRepo.On("GetByID", s.ctx, sessionID).Return(expectedSession, nil)

	session, err := s.service.GetSession(s.ctx, sessionID)

	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), session)
	assert.Equal(s.T(), sessionID, session.ID)
}

func (s *SessionServiceTestSuite) TestGetSession_NotFound() {
	sessionID := 999

	s.mockRepo.On("GetByID", s.ctx, sessionID).Return(nil, database.ErrNotFound)

	session, err := s.service.GetSession(s.ctx, sessionID)

	assert.Error(s.T(), err)
	assert.Nil(s.T(), session)
	assert.Equal(s.T(), ErrSessionNotFound, err)
}

// ValidateToken tests

func (s *SessionServiceTestSuite) TestValidateToken_Success() {
	token := "valid-token"
	expectedSession := &models.Session{
		ID:        1,
		UserID:    1,
		Token:     token,
		ExpiresAt: time.Now().Add(time.Hour),
	}

	s.mockRepo.On("GetByToken", s.ctx, token).Return(expectedSession, nil)

	session, err := s.service.ValidateToken(s.ctx, token)

	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), session)
	assert.Equal(s.T(), token, session.Token)
}

func (s *SessionServiceTestSuite) TestValidateToken_EmptyToken() {
	session, err := s.service.ValidateToken(s.ctx, "")

	assert.Error(s.T(), err)
	assert.Nil(s.T(), session)
	assert.Equal(s.T(), ErrInvalidToken, err)
}

func (s *SessionServiceTestSuite) TestValidateToken_NotFound() {
	token := "invalid-token"

	s.mockRepo.On("GetByToken", s.ctx, token).Return(nil, database.ErrNotFound)

	session, err := s.service.ValidateToken(s.ctx, token)

	assert.Error(s.T(), err)
	assert.Nil(s.T(), session)
	assert.Equal(s.T(), ErrSessionNotFound, err)
}

func (s *SessionServiceTestSuite) TestValidateToken_Expired() {
	token := "expired-token"
	expiredSession := &models.Session{
		ID:        1,
		UserID:    1,
		Token:     token,
		ExpiresAt: time.Now().Add(-time.Hour), // expired
	}

	s.mockRepo.On("GetByToken", s.ctx, token).Return(expiredSession, nil)

	session, err := s.service.ValidateToken(s.ctx, token)

	assert.Error(s.T(), err)
	assert.Nil(s.T(), session)
	assert.Equal(s.T(), ErrSessionExpired, err)
}

// GetUserSessions tests

func (s *SessionServiceTestSuite) TestGetUserSessions_Success() {
	userID := 1
	expectedSessions := models.SessionSlice{
		{ID: 1, UserID: userID, Token: "token1"},
		{ID: 2, UserID: userID, Token: "token2"},
	}

	s.mockRepo.On("GetByUserID", s.ctx, userID).Return(expectedSessions, nil)

	sessions, err := s.service.GetUserSessions(s.ctx, userID)

	assert.NoError(s.T(), err)
	assert.Len(s.T(), sessions, 2)
}

func (s *SessionServiceTestSuite) TestGetUserSessions_Empty() {
	userID := 1
	expectedSessions := models.SessionSlice{}

	s.mockRepo.On("GetByUserID", s.ctx, userID).Return(expectedSessions, nil)

	sessions, err := s.service.GetUserSessions(s.ctx, userID)

	assert.NoError(s.T(), err)
	assert.Empty(s.T(), sessions)
}

// RefreshSession tests

func (s *SessionServiceTestSuite) TestRefreshSession_Success() {
	sessionID := 1
	existingSession := &models.Session{
		ID:        sessionID,
		UserID:    1,
		Token:     "test-token",
		ExpiresAt: time.Now().Add(10 * time.Minute),
	}

	s.mockRepo.On("GetByID", s.ctx, sessionID).Return(existingSession, nil)
	s.mockRepo.On("Update", s.ctx, mock.AnythingOfType("*models.Session")).Return(nil)

	session, err := s.service.RefreshSession(s.ctx, sessionID)

	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), session)
	assert.True(s.T(), session.ExpiresAt.After(time.Now().Add(50*time.Minute)))
}

func (s *SessionServiceTestSuite) TestRefreshSession_NotFound() {
	sessionID := 999

	s.mockRepo.On("GetByID", s.ctx, sessionID).Return(nil, database.ErrNotFound)

	session, err := s.service.RefreshSession(s.ctx, sessionID)

	assert.Error(s.T(), err)
	assert.Nil(s.T(), session)
	assert.Equal(s.T(), ErrSessionNotFound, err)
}

// RefreshSessionByToken tests

func (s *SessionServiceTestSuite) TestRefreshSessionByToken_Success() {
	token := "test-token"
	existingSession := &models.Session{
		ID:        1,
		UserID:    1,
		Token:     token,
		ExpiresAt: time.Now().Add(10 * time.Minute),
	}

	s.mockRepo.On("GetByToken", s.ctx, token).Return(existingSession, nil)
	s.mockRepo.On("Update", s.ctx, mock.AnythingOfType("*models.Session")).Return(nil)

	session, err := s.service.RefreshSessionByToken(s.ctx, token)

	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), session)
}

func (s *SessionServiceTestSuite) TestRefreshSessionByToken_NotFound() {
	token := "invalid-token"

	s.mockRepo.On("GetByToken", s.ctx, token).Return(nil, database.ErrNotFound)

	session, err := s.service.RefreshSessionByToken(s.ctx, token)

	assert.Error(s.T(), err)
	assert.Nil(s.T(), session)
	assert.Equal(s.T(), ErrSessionNotFound, err)
}

// EndSession tests

func (s *SessionServiceTestSuite) TestEndSession_Success() {
	sessionID := 1

	s.mockRepo.On("Delete", s.ctx, sessionID).Return(nil)

	err := s.service.EndSession(s.ctx, sessionID)

	assert.NoError(s.T(), err)
}

func (s *SessionServiceTestSuite) TestEndSession_NotFound() {
	sessionID := 999

	s.mockRepo.On("Delete", s.ctx, sessionID).Return(database.ErrNotFound)

	err := s.service.EndSession(s.ctx, sessionID)

	assert.Error(s.T(), err)
	assert.Equal(s.T(), ErrSessionNotFound, err)
}

// EndSessionByToken tests

func (s *SessionServiceTestSuite) TestEndSessionByToken_Success() {
	token := "test-token"

	s.mockRepo.On("DeleteByToken", s.ctx, token).Return(nil)

	err := s.service.EndSessionByToken(s.ctx, token)

	assert.NoError(s.T(), err)
}

func (s *SessionServiceTestSuite) TestEndSessionByToken_NotFound() {
	token := "invalid-token"

	s.mockRepo.On("DeleteByToken", s.ctx, token).Return(database.ErrNotFound)

	err := s.service.EndSessionByToken(s.ctx, token)

	assert.Error(s.T(), err)
	assert.Equal(s.T(), ErrSessionNotFound, err)
}

// EndAllUserSessions tests

func (s *SessionServiceTestSuite) TestEndAllUserSessions_Success() {
	userID := 1
	deletedCount := int64(3)

	s.mockRepo.On("DeleteByUserID", s.ctx, userID).Return(deletedCount, nil)

	count, err := s.service.EndAllUserSessions(s.ctx, userID)

	assert.NoError(s.T(), err)
	assert.Equal(s.T(), deletedCount, count)
}

// CleanupExpired tests

func (s *SessionServiceTestSuite) TestCleanupExpired_Success() {
	deletedCount := int64(5)

	s.mockRepo.On("DeleteExpired", s.ctx).Return(deletedCount, nil)

	count, err := s.service.CleanupExpired(s.ctx)

	assert.NoError(s.T(), err)
	assert.Equal(s.T(), deletedCount, count)
}

// WithSessionDuration option test

func TestNewSessionService_WithCustomDuration(t *testing.T) {
	mockRepo := mocks.NewMockSessionRepo()
	customDuration := 48 * time.Hour

	service := NewSessionService(mockRepo, WithSessionDuration(customDuration))

	assert.Equal(t, customDuration, service.sessionDuration)
}

func TestNewSessionService_DefaultDuration(t *testing.T) {
	mockRepo := mocks.NewMockSessionRepo()

	service := NewSessionService(mockRepo)

	assert.Equal(t, DefaultSessionDuration, service.sessionDuration)
}
