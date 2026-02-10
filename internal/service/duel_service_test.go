package service

import (
	"context"
	"errors"
	"testing"

	"bytebattle/internal/database"
	"bytebattle/internal/database/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockDuelRepo struct {
	createFunc        func(ctx context.Context, players []database.Player, problemID int) (*models.Duel, error)
	getByIDFunc       func(ctx context.Context, id int) (*models.Duel, error)
	getAllFunc        func(ctx context.Context, limit, offset int) (models.DuelSlice, error)
	upsertFunc        func(ctx context.Context, duel *models.Duel) error
	deleteFunc        func(ctx context.Context, id int) error
	getByPlayerIDFunc func(ctx context.Context, playerID int) (models.DuelSlice, error)
	getByStatusFunc   func(ctx context.Context, status database.DuelStatus) (models.DuelSlice, error)
}

func (m *mockDuelRepo) Create(ctx context.Context, players []database.Player, problemID int) (*models.Duel, error) {
	if m.createFunc != nil {
		return m.createFunc(ctx, players, problemID)
	}
	return &models.Duel{ID: 1}, nil
}

func (m *mockDuelRepo) GetByID(ctx context.Context, id int) (*models.Duel, error) {
	if m.getByIDFunc != nil {
		return m.getByIDFunc(ctx, id)
	}
	return &models.Duel{ID: id, Status: string(database.DuelStatusPending)}, nil
}

func (m *mockDuelRepo) GetAll(ctx context.Context, limit, offset int) (models.DuelSlice, error) {
	if m.getAllFunc != nil {
		return m.getAllFunc(ctx, limit, offset)
	}
	return models.DuelSlice{}, nil
}

func (m *mockDuelRepo) Upsert(ctx context.Context, duel *models.Duel) error {
	if m.upsertFunc != nil {
		return m.upsertFunc(ctx, duel)
	}
	return nil
}

func (m *mockDuelRepo) Delete(ctx context.Context, id int) error {
	if m.deleteFunc != nil {
		return m.deleteFunc(ctx, id)
	}
	return nil
}

func (m *mockDuelRepo) GetByPlayerID(ctx context.Context, playerID int) (models.DuelSlice, error) {
	if m.getByPlayerIDFunc != nil {
		return m.getByPlayerIDFunc(ctx, playerID)
	}
	return models.DuelSlice{}, nil
}

func (m *mockDuelRepo) GetByStatus(ctx context.Context, status database.DuelStatus) (models.DuelSlice, error) {
	if m.getByStatusFunc != nil {
		return m.getByStatusFunc(ctx, status)
	}
	return models.DuelSlice{}, nil
}

func TestCreateDuel_Success(t *testing.T) {
	mock := &mockDuelRepo{
		createFunc: func(ctx context.Context, players []database.Player, problemID int) (*models.Duel, error) {
			return &models.Duel{
				ID:        1,
				Player1ID: players[0].ID,
				Player2ID: players[1].ID,
				ProblemID: problemID,
				Status:    string(database.DuelStatusPending),
			}, nil
		},
	}

	svc := NewDuelService(mock)
	duel, err := svc.CreateDuel(context.Background(), []int{1, 2}, 10)

	require.NoError(t, err)
	assert.Equal(t, 1, duel.ID)
	assert.Equal(t, 1, duel.Player1ID)
	assert.Equal(t, 2, duel.Player2ID)
}

func TestCreateDuel_ThreePlayers(t *testing.T) {
	var capturedPlayers []database.Player
	mock := &mockDuelRepo{
		createFunc: func(ctx context.Context, players []database.Player, problemID int) (*models.Duel, error) {
			capturedPlayers = players
			return &models.Duel{ID: 1}, nil
		},
	}

	svc := NewDuelService(mock)
	_, err := svc.CreateDuel(context.Background(), []int{1, 2, 3}, 10)

	require.NoError(t, err)
	assert.Len(t, capturedPlayers, 3)
}

func TestCreateDuel_NotEnoughPlayers(t *testing.T) {
	mock := &mockDuelRepo{}
	svc := NewDuelService(mock)

	_, err := svc.CreateDuel(context.Background(), []int{1}, 10)

	require.Error(t, err)
	assert.EqualError(t, err, "at least two players are required")
}

func TestCreateDuel_EmptyPlayers(t *testing.T) {
	mock := &mockDuelRepo{}
	svc := NewDuelService(mock)

	_, err := svc.CreateDuel(context.Background(), []int{}, 10)

	require.Error(t, err)
}

func TestCreateDuel_DuplicatePlayers(t *testing.T) {
	mock := &mockDuelRepo{}
	svc := NewDuelService(mock)

	_, err := svc.CreateDuel(context.Background(), []int{1, 1}, 10)

	require.Error(t, err)
	assert.EqualError(t, err, "players must be different")
}

func TestCreateDuel_DuplicateInThree(t *testing.T) {
	mock := &mockDuelRepo{}
	svc := NewDuelService(mock)

	_, err := svc.CreateDuel(context.Background(), []int{1, 2, 1}, 10)

	require.Error(t, err)
}

func TestCreateDuel_RepoError(t *testing.T) {
	mock := &mockDuelRepo{
		createFunc: func(ctx context.Context, players []database.Player, problemID int) (*models.Duel, error) {
			return nil, errors.New("database error")
		},
	}

	svc := NewDuelService(mock)
	_, err := svc.CreateDuel(context.Background(), []int{1, 2}, 10)

	require.Error(t, err)
}

func TestGetDuel_Success(t *testing.T) {
	mock := &mockDuelRepo{
		getByIDFunc: func(ctx context.Context, id int) (*models.Duel, error) {
			return &models.Duel{ID: id}, nil
		},
	}

	svc := NewDuelService(mock)
	duel, err := svc.GetDuel(context.Background(), 5)

	require.NoError(t, err)
	assert.Equal(t, 5, duel.ID)
}

func TestGetDuel_NotFound(t *testing.T) {
	mock := &mockDuelRepo{
		getByIDFunc: func(ctx context.Context, id int) (*models.Duel, error) {
			return nil, errors.New("not found")
		},
	}

	svc := NewDuelService(mock)
	_, err := svc.GetDuel(context.Background(), 999)

	require.Error(t, err)
}

func TestListDuels_DefaultLimit(t *testing.T) {
	var capturedLimit int
	mock := &mockDuelRepo{
		getAllFunc: func(ctx context.Context, limit, offset int) (models.DuelSlice, error) {
			capturedLimit = limit
			return models.DuelSlice{}, nil
		},
	}

	svc := NewDuelService(mock)
	_, _ = svc.ListDuels(context.Background(), 0, 0)

	assert.Equal(t, 10, capturedLimit)
}

func TestListDuels_NegativeLimit(t *testing.T) {
	var capturedLimit int
	mock := &mockDuelRepo{
		getAllFunc: func(ctx context.Context, limit, offset int) (models.DuelSlice, error) {
			capturedLimit = limit
			return models.DuelSlice{}, nil
		},
	}

	svc := NewDuelService(mock)
	_, _ = svc.ListDuels(context.Background(), -5, 0)

	assert.Equal(t, 10, capturedLimit)
}

func TestListDuels_MaxLimit(t *testing.T) {
	var capturedLimit int
	mock := &mockDuelRepo{
		getAllFunc: func(ctx context.Context, limit, offset int) (models.DuelSlice, error) {
			capturedLimit = limit
			return models.DuelSlice{}, nil
		},
	}

	svc := NewDuelService(mock)
	_, _ = svc.ListDuels(context.Background(), 500, 0)

	assert.Equal(t, 100, capturedLimit)
}

func TestListDuels_ValidLimit(t *testing.T) {
	var capturedLimit int
	mock := &mockDuelRepo{
		getAllFunc: func(ctx context.Context, limit, offset int) (models.DuelSlice, error) {
			capturedLimit = limit
			return models.DuelSlice{}, nil
		},
	}

	svc := NewDuelService(mock)
	_, _ = svc.ListDuels(context.Background(), 50, 0)

	assert.Equal(t, 50, capturedLimit)
}

func TestStartDuel_Success(t *testing.T) {
	mock := &mockDuelRepo{
		getByIDFunc: func(ctx context.Context, id int) (*models.Duel, error) {
			return &models.Duel{ID: id, Status: string(database.DuelStatusPending)}, nil
		},
		upsertFunc: func(ctx context.Context, duel *models.Duel) error {
			return nil
		},
	}

	svc := NewDuelService(mock)
	duel, err := svc.StartDuel(context.Background(), 1)

	require.NoError(t, err)
	assert.Equal(t, string(database.DuelStatusActive), duel.Status)
}

func TestStartDuel_AlreadyActive(t *testing.T) {
	mock := &mockDuelRepo{
		getByIDFunc: func(ctx context.Context, id int) (*models.Duel, error) {
			return &models.Duel{ID: id, Status: string(database.DuelStatusActive)}, nil
		},
	}

	svc := NewDuelService(mock)
	_, err := svc.StartDuel(context.Background(), 1)

	require.Error(t, err)
}

func TestStartDuel_AlreadyFinished(t *testing.T) {
	mock := &mockDuelRepo{
		getByIDFunc: func(ctx context.Context, id int) (*models.Duel, error) {
			return &models.Duel{ID: id, Status: string(database.DuelStatusFinished)}, nil
		},
	}

	svc := NewDuelService(mock)
	_, err := svc.StartDuel(context.Background(), 1)

	require.Error(t, err)
}

func TestStartDuel_NotFound(t *testing.T) {
	mock := &mockDuelRepo{
		getByIDFunc: func(ctx context.Context, id int) (*models.Duel, error) {
			return nil, errors.New("not found")
		},
	}

	svc := NewDuelService(mock)
	_, err := svc.StartDuel(context.Background(), 1)

	require.Error(t, err)
}

func TestStartDuel_UpsertError(t *testing.T) {
	mock := &mockDuelRepo{
		getByIDFunc: func(ctx context.Context, id int) (*models.Duel, error) {
			return &models.Duel{ID: id, Status: string(database.DuelStatusPending)}, nil
		},
		upsertFunc: func(ctx context.Context, duel *models.Duel) error {
			return errors.New("upsert error")
		},
	}

	svc := NewDuelService(mock)
	_, err := svc.StartDuel(context.Background(), 1)

	require.Error(t, err)
}

func TestCompleteDuel_Success(t *testing.T) {
	mock := &mockDuelRepo{
		getByIDFunc: func(ctx context.Context, id int) (*models.Duel, error) {
			return &models.Duel{
				ID:        id,
				Status:    string(database.DuelStatusActive),
				Player1ID: 1,
				Player2ID: 2,
			}, nil
		},
		upsertFunc: func(ctx context.Context, duel *models.Duel) error {
			return nil
		},
	}

	svc := NewDuelService(mock)
	duel, err := svc.CompleteDuel(context.Background(), 1, 1)

	require.NoError(t, err)
	assert.Equal(t, string(database.DuelStatusFinished), duel.Status)
	assert.True(t, duel.WinnerID.Valid)
	assert.Equal(t, 1, duel.WinnerID.Int)
}

func TestCompleteDuel_Player2Wins(t *testing.T) {
	mock := &mockDuelRepo{
		getByIDFunc: func(ctx context.Context, id int) (*models.Duel, error) {
			return &models.Duel{
				ID:        id,
				Status:    string(database.DuelStatusActive),
				Player1ID: 1,
				Player2ID: 2,
			}, nil
		},
		upsertFunc: func(ctx context.Context, duel *models.Duel) error {
			return nil
		},
	}

	svc := NewDuelService(mock)
	duel, err := svc.CompleteDuel(context.Background(), 1, 2)

	require.NoError(t, err)
	assert.True(t, duel.WinnerID.Valid)
	assert.Equal(t, 2, duel.WinnerID.Int)
}

func TestCompleteDuel_InvalidWinner(t *testing.T) {
	mock := &mockDuelRepo{
		getByIDFunc: func(ctx context.Context, id int) (*models.Duel, error) {
			return &models.Duel{
				ID:        id,
				Status:    string(database.DuelStatusActive),
				Player1ID: 1,
				Player2ID: 2,
			}, nil
		},
	}

	svc := NewDuelService(mock)
	_, err := svc.CompleteDuel(context.Background(), 1, 999)

	require.Error(t, err)
	assert.EqualError(t, err, "winner must be one of the players")
}

func TestCompleteDuel_NotInProgress(t *testing.T) {
	mock := &mockDuelRepo{
		getByIDFunc: func(ctx context.Context, id int) (*models.Duel, error) {
			return &models.Duel{ID: id, Status: string(database.DuelStatusPending)}, nil
		},
	}

	svc := NewDuelService(mock)
	_, err := svc.CompleteDuel(context.Background(), 1, 1)

	require.Error(t, err)
}

func TestCancelDuel_Success(t *testing.T) {
	mock := &mockDuelRepo{
		getByIDFunc: func(ctx context.Context, id int) (*models.Duel, error) {
			return &models.Duel{ID: id, Status: string(database.DuelStatusPending)}, nil
		},
		upsertFunc: func(ctx context.Context, duel *models.Duel) error {
			return nil
		},
	}

	svc := NewDuelService(mock)
	duel, err := svc.CancelDuel(context.Background(), 1)

	require.NoError(t, err)
	assert.Equal(t, string(database.DuelStatusCancelled), duel.Status)
}

func TestCancelDuel_ActiveDuel(t *testing.T) {
	mock := &mockDuelRepo{
		getByIDFunc: func(ctx context.Context, id int) (*models.Duel, error) {
			return &models.Duel{ID: id, Status: string(database.DuelStatusActive)}, nil
		},
		upsertFunc: func(ctx context.Context, duel *models.Duel) error {
			return nil
		},
	}

	svc := NewDuelService(mock)
	duel, err := svc.CancelDuel(context.Background(), 1)

	require.NoError(t, err)
	assert.Equal(t, string(database.DuelStatusCancelled), duel.Status)
}

func TestCancelDuel_AlreadyCompleted(t *testing.T) {
	mock := &mockDuelRepo{
		getByIDFunc: func(ctx context.Context, id int) (*models.Duel, error) {
			return &models.Duel{ID: id, Status: string(database.DuelStatusFinished)}, nil
		},
	}

	svc := NewDuelService(mock)
	_, err := svc.CancelDuel(context.Background(), 1)

	require.Error(t, err)
}

func TestDeleteDuel_Success(t *testing.T) {
	mock := &mockDuelRepo{
		deleteFunc: func(ctx context.Context, id int) error {
			return nil
		},
	}

	svc := NewDuelService(mock)
	err := svc.DeleteDuel(context.Background(), 1)

	require.NoError(t, err)
}

func TestDeleteDuel_Error(t *testing.T) {
	mock := &mockDuelRepo{
		deleteFunc: func(ctx context.Context, id int) error {
			return errors.New("delete error")
		},
	}

	svc := NewDuelService(mock)
	err := svc.DeleteDuel(context.Background(), 1)

	require.Error(t, err)
}

func TestGetPlayerDuels_Success(t *testing.T) {
	mock := &mockDuelRepo{
		getByPlayerIDFunc: func(ctx context.Context, playerID int) (models.DuelSlice, error) {
			return models.DuelSlice{&models.Duel{ID: 1}, &models.Duel{ID: 2}}, nil
		},
	}

	svc := NewDuelService(mock)
	duels, err := svc.GetPlayerDuels(context.Background(), 1)

	require.NoError(t, err)
	assert.Len(t, duels, 2)
}

func TestGetActiveDuels_Success(t *testing.T) {
	var capturedStatus database.DuelStatus
	mock := &mockDuelRepo{
		getByStatusFunc: func(ctx context.Context, status database.DuelStatus) (models.DuelSlice, error) {
			capturedStatus = status
			return models.DuelSlice{&models.Duel{ID: 1}}, nil
		},
	}

	svc := NewDuelService(mock)
	duels, err := svc.GetActiveDuels(context.Background())

	require.NoError(t, err)
	assert.Len(t, duels, 1)
	assert.Equal(t, database.DuelStatusActive, capturedStatus)
}
