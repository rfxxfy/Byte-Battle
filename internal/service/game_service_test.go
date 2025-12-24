package service

import (
	"context"
	"errors"
	"testing"

	"bytebattle/internal/database"
	"bytebattle/internal/database/models"

	"github.com/aarondl/null/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockGameRepo struct {
	createFunc       func(ctx context.Context, players []database.Player, problemID int) (*models.Game, error)
	getByIDFunc      func(ctx context.Context, id int) (*models.Game, error)
	getAllFunc        func(ctx context.Context, limit, offset int) (models.GameSlice, error)
	countFunc        func(ctx context.Context) (int64, error)
	startGameFunc    func(ctx context.Context, id int) (*models.Game, error)
	completeGameFunc func(ctx context.Context, id, winnerID int) (*models.Game, error)
	cancelGameFunc   func(ctx context.Context, id int) (*models.Game, error)
	deleteFunc       func(ctx context.Context, id int) error
}

func (m *mockGameRepo) Create(ctx context.Context, players []database.Player, problemID int) (*models.Game, error) {
	if m.createFunc != nil {
		return m.createFunc(ctx, players, problemID)
	}
	return &models.Game{ID: 1}, nil
}

func (m *mockGameRepo) GetByID(ctx context.Context, id int) (*models.Game, error) {
	if m.getByIDFunc != nil {
		return m.getByIDFunc(ctx, id)
	}
	return &models.Game{ID: id, Status: database.GameStatusPending}, nil
}

func (m *mockGameRepo) GetAll(ctx context.Context, limit, offset int) (models.GameSlice, error) {
	if m.getAllFunc != nil {
		return m.getAllFunc(ctx, limit, offset)
	}
	return models.GameSlice{}, nil
}

func (m *mockGameRepo) Count(ctx context.Context) (int64, error) {
	if m.countFunc != nil {
		return m.countFunc(ctx)
	}
	return 0, nil
}

func (m *mockGameRepo) StartGame(ctx context.Context, id int) (*models.Game, error) {
	if m.startGameFunc != nil {
		return m.startGameFunc(ctx, id)
	}
	return &models.Game{ID: id, Status: database.GameStatusActive}, nil
}

func (m *mockGameRepo) CompleteGame(ctx context.Context, id, winnerID int) (*models.Game, error) {
	if m.completeGameFunc != nil {
		return m.completeGameFunc(ctx, id, winnerID)
	}
	return &models.Game{ID: id, Status: database.GameStatusFinished, WinnerID: null.IntFrom(winnerID)}, nil
}

func (m *mockGameRepo) CancelGame(ctx context.Context, id int) (*models.Game, error) {
	if m.cancelGameFunc != nil {
		return m.cancelGameFunc(ctx, id)
	}
	return &models.Game{ID: id, Status: database.GameStatusCancelled}, nil
}

func (m *mockGameRepo) Delete(ctx context.Context, id int) error {
	if m.deleteFunc != nil {
		return m.deleteFunc(ctx, id)
	}
	return nil
}

func TestCreateGame_Success(t *testing.T) {
	mock := &mockGameRepo{
		createFunc: func(ctx context.Context, players []database.Player, problemID int) (*models.Game, error) {
			return &models.Game{
				ID:        1,
				ProblemID: problemID,
				Status:    database.GameStatusPending,
			}, nil
		},
	}

	svc := NewGameService(mock)
	game, err := svc.CreateGame(context.Background(), []int{1, 2}, 10)

	require.NoError(t, err)
	assert.Equal(t, 1, game.ID)
}

func TestCreateGame_ThreePlayers(t *testing.T) {
	var capturedPlayers []database.Player
	mock := &mockGameRepo{
		createFunc: func(ctx context.Context, players []database.Player, problemID int) (*models.Game, error) {
			capturedPlayers = players
			return &models.Game{ID: 1}, nil
		},
	}

	svc := NewGameService(mock)
	_, err := svc.CreateGame(context.Background(), []int{1, 2, 3}, 10)

	require.NoError(t, err)
	assert.Len(t, capturedPlayers, 3)
}

func TestCreateGame_NotEnoughPlayers(t *testing.T) {
	mock := &mockGameRepo{}
	svc := NewGameService(mock)

	_, err := svc.CreateGame(context.Background(), []int{1}, 10)

	require.Error(t, err)
	assert.EqualError(t, err, "at least two players are required")
}

func TestCreateGame_EmptyPlayers(t *testing.T) {
	mock := &mockGameRepo{}
	svc := NewGameService(mock)

	_, err := svc.CreateGame(context.Background(), []int{}, 10)

	require.Error(t, err)
}

func TestCreateGame_DuplicatePlayers(t *testing.T) {
	mock := &mockGameRepo{}
	svc := NewGameService(mock)

	_, err := svc.CreateGame(context.Background(), []int{1, 1}, 10)

	require.Error(t, err)
	assert.EqualError(t, err, "players must be different")
}

func TestCreateGame_DuplicateInThree(t *testing.T) {
	mock := &mockGameRepo{}
	svc := NewGameService(mock)

	_, err := svc.CreateGame(context.Background(), []int{1, 2, 1}, 10)

	require.Error(t, err)
}

func TestCreateGame_RepoError(t *testing.T) {
	mock := &mockGameRepo{
		createFunc: func(ctx context.Context, players []database.Player, problemID int) (*models.Game, error) {
			return nil, errors.New("database error")
		},
	}

	svc := NewGameService(mock)
	_, err := svc.CreateGame(context.Background(), []int{1, 2}, 10)

	require.Error(t, err)
}

func TestGetGame_Success(t *testing.T) {
	mock := &mockGameRepo{
		getByIDFunc: func(ctx context.Context, id int) (*models.Game, error) {
			return &models.Game{ID: id}, nil
		},
	}

	svc := NewGameService(mock)
	game, err := svc.GetGame(context.Background(), 5)

	require.NoError(t, err)
	assert.Equal(t, 5, game.ID)
}

func TestGetGame_NotFound(t *testing.T) {
	mock := &mockGameRepo{
		getByIDFunc: func(ctx context.Context, id int) (*models.Game, error) {
			return nil, errors.New("not found")
		},
	}

	svc := NewGameService(mock)
	_, err := svc.GetGame(context.Background(), 999)

	require.Error(t, err)
}

func TestGetGame_SqlNoRows_ReturnsErrGameNotFound(t *testing.T) {
	mock := &mockGameRepo{
		getByIDFunc: func(ctx context.Context, id int) (*models.Game, error) {
			return nil, database.ErrNotFound
		},
	}

	svc := NewGameService(mock)
	_, err := svc.GetGame(context.Background(), 999)

	require.ErrorIs(t, err, ErrGameNotFound)
}

func TestStartGame_Success(t *testing.T) {
	mock := &mockGameRepo{
		startGameFunc: func(ctx context.Context, id int) (*models.Game, error) {
			return &models.Game{ID: id, Status: database.GameStatusActive}, nil
		},
	}

	svc := NewGameService(mock)
	game, err := svc.StartGame(context.Background(), 1)

	require.NoError(t, err)
	assert.Equal(t, database.GameStatusActive, game.Status)
}

func TestStartGame_AlreadyActive(t *testing.T) {
	mock := &mockGameRepo{
		startGameFunc: func(ctx context.Context, id int) (*models.Game, error) {
			return nil, database.ErrGameNotPending
		},
	}

	svc := NewGameService(mock)
	_, err := svc.StartGame(context.Background(), 1)

	require.ErrorIs(t, err, ErrGameAlreadyStarted)
}

func TestStartGame_AlreadyFinished(t *testing.T) {
	mock := &mockGameRepo{
		startGameFunc: func(ctx context.Context, id int) (*models.Game, error) {
			return nil, database.ErrGameNotPending
		},
	}

	svc := NewGameService(mock)
	_, err := svc.StartGame(context.Background(), 1)

	require.ErrorIs(t, err, ErrGameAlreadyStarted)
}

func TestStartGame_NotFound(t *testing.T) {
	mock := &mockGameRepo{
		startGameFunc: func(ctx context.Context, id int) (*models.Game, error) {
			return nil, database.ErrNotFound
		},
	}

	svc := NewGameService(mock)
	_, err := svc.StartGame(context.Background(), 1)

	require.ErrorIs(t, err, ErrGameNotFound)
}

func TestStartGame_SqlNoRows_ReturnsErrGameNotFound(t *testing.T) {
	mock := &mockGameRepo{
		startGameFunc: func(ctx context.Context, id int) (*models.Game, error) {
			return nil, database.ErrNotFound
		},
	}

	svc := NewGameService(mock)
	_, err := svc.StartGame(context.Background(), 999)

	require.ErrorIs(t, err, ErrGameNotFound)
}

func TestStartGame_RepoError(t *testing.T) {
	mock := &mockGameRepo{
		startGameFunc: func(ctx context.Context, id int) (*models.Game, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewGameService(mock)
	_, err := svc.StartGame(context.Background(), 1)

	require.Error(t, err)
}

func TestDeleteGame_SqlNoRows_ReturnsErrGameNotFound(t *testing.T) {
	mock := &mockGameRepo{
		deleteFunc: func(ctx context.Context, id int) error {
			return database.ErrNotFound
		},
	}

	svc := NewGameService(mock)
	err := svc.DeleteGame(context.Background(), 999)

	require.ErrorIs(t, err, ErrGameNotFound)
}

func TestListGames_DefaultLimit(t *testing.T) {
	var capturedLimit int
	mock := &mockGameRepo{
		getAllFunc: func(ctx context.Context, limit, offset int) (models.GameSlice, error) {
			capturedLimit = limit
			return models.GameSlice{}, nil
		},
	}

	svc := NewGameService(mock)
	_, _, _ = svc.ListGames(context.Background(), 0, 0)

	assert.Equal(t, 10, capturedLimit)
}

func TestListGames_NegativeLimit(t *testing.T) {
	var capturedLimit int
	mock := &mockGameRepo{
		getAllFunc: func(ctx context.Context, limit, offset int) (models.GameSlice, error) {
			capturedLimit = limit
			return models.GameSlice{}, nil
		},
	}

	svc := NewGameService(mock)
	_, _, _ = svc.ListGames(context.Background(), -5, 0)

	assert.Equal(t, 10, capturedLimit)
}

func TestListGames_MaxLimit(t *testing.T) {
	var capturedLimit int
	mock := &mockGameRepo{
		getAllFunc: func(ctx context.Context, limit, offset int) (models.GameSlice, error) {
			capturedLimit = limit
			return models.GameSlice{}, nil
		},
	}

	svc := NewGameService(mock)
	_, _, _ = svc.ListGames(context.Background(), 500, 0)

	assert.Equal(t, 100, capturedLimit)
}

func TestListGames_ValidLimit(t *testing.T) {
	var capturedLimit int
	mock := &mockGameRepo{
		getAllFunc: func(ctx context.Context, limit, offset int) (models.GameSlice, error) {
			capturedLimit = limit
			return models.GameSlice{}, nil
		},
	}

	svc := NewGameService(mock)
	_, _, _ = svc.ListGames(context.Background(), 50, 0)

	assert.Equal(t, 50, capturedLimit)
}

func TestCompleteGame_Success(t *testing.T) {
	mock := &mockGameRepo{
		completeGameFunc: func(ctx context.Context, id, winnerID int) (*models.Game, error) {
			return &models.Game{
				ID:       id,
				Status:   database.GameStatusFinished,
				WinnerID: null.IntFrom(winnerID),
			}, nil
		},
	}

	svc := NewGameService(mock)
	game, err := svc.CompleteGame(context.Background(), 1, 1)

	require.NoError(t, err)
	assert.Equal(t, database.GameStatusFinished, game.Status)
	assert.True(t, game.WinnerID.Valid)
	assert.Equal(t, 1, game.WinnerID.Int)
}

func TestCompleteGame_SecondPlayerWins(t *testing.T) {
	mock := &mockGameRepo{
		completeGameFunc: func(ctx context.Context, id, winnerID int) (*models.Game, error) {
			return &models.Game{
				ID:       id,
				Status:   database.GameStatusFinished,
				WinnerID: null.IntFrom(winnerID),
			}, nil
		},
	}

	svc := NewGameService(mock)
	game, err := svc.CompleteGame(context.Background(), 1, 2)

	require.NoError(t, err)
	assert.True(t, game.WinnerID.Valid)
	assert.Equal(t, 2, game.WinnerID.Int)
}

func TestCompleteGame_InvalidWinner(t *testing.T) {
	mock := &mockGameRepo{
		completeGameFunc: func(ctx context.Context, id, winnerID int) (*models.Game, error) {
			return nil, database.ErrNotParticipant
		},
	}

	svc := NewGameService(mock)
	_, err := svc.CompleteGame(context.Background(), 1, 999)

	require.ErrorIs(t, err, ErrInvalidWinner)
	assert.EqualError(t, err, "winner must be one of the players")
}

func TestCompleteGame_NotInProgress(t *testing.T) {
	mock := &mockGameRepo{
		completeGameFunc: func(ctx context.Context, id, winnerID int) (*models.Game, error) {
			return nil, database.ErrGameNotActive
		},
	}

	svc := NewGameService(mock)
	_, err := svc.CompleteGame(context.Background(), 1, 1)

	require.ErrorIs(t, err, ErrGameNotInProgress)
}

func TestCompleteGame_NotFound(t *testing.T) {
	mock := &mockGameRepo{
		completeGameFunc: func(ctx context.Context, id, winnerID int) (*models.Game, error) {
			return nil, database.ErrNotFound
		},
	}

	svc := NewGameService(mock)
	_, err := svc.CompleteGame(context.Background(), 999, 1)

	require.ErrorIs(t, err, ErrGameNotFound)
}

func TestCancelGame_Success(t *testing.T) {
	mock := &mockGameRepo{
		cancelGameFunc: func(ctx context.Context, id int) (*models.Game, error) {
			return &models.Game{ID: id, Status: database.GameStatusCancelled}, nil
		},
	}

	svc := NewGameService(mock)
	game, err := svc.CancelGame(context.Background(), 1)

	require.NoError(t, err)
	assert.Equal(t, database.GameStatusCancelled, game.Status)
}

func TestCancelGame_ActiveGame(t *testing.T) {
	mock := &mockGameRepo{
		cancelGameFunc: func(ctx context.Context, id int) (*models.Game, error) {
			return &models.Game{ID: id, Status: database.GameStatusCancelled}, nil
		},
	}

	svc := NewGameService(mock)
	game, err := svc.CancelGame(context.Background(), 1)

	require.NoError(t, err)
	assert.Equal(t, database.GameStatusCancelled, game.Status)
}

func TestCancelGame_AlreadyFinished(t *testing.T) {
	mock := &mockGameRepo{
		cancelGameFunc: func(ctx context.Context, id int) (*models.Game, error) {
			return nil, database.ErrGameFinished
		},
	}

	svc := NewGameService(mock)
	_, err := svc.CancelGame(context.Background(), 1)

	require.ErrorIs(t, err, ErrCannotCancelFinished)
}

func TestCancelGame_AlreadyCancelled(t *testing.T) {
	mock := &mockGameRepo{
		cancelGameFunc: func(ctx context.Context, id int) (*models.Game, error) {
			return nil, database.ErrGameAlreadyCancelled
		},
	}

	svc := NewGameService(mock)
	_, err := svc.CancelGame(context.Background(), 1)

	require.ErrorIs(t, err, ErrGameAlreadyCancelled)
}

func TestDeleteGame_Success(t *testing.T) {
	mock := &mockGameRepo{
		deleteFunc: func(ctx context.Context, id int) error {
			return nil
		},
	}

	svc := NewGameService(mock)
	err := svc.DeleteGame(context.Background(), 1)

	require.NoError(t, err)
}

func TestDeleteGame_Error(t *testing.T) {
	mock := &mockGameRepo{
		deleteFunc: func(ctx context.Context, id int) error {
			return errors.New("delete error")
		},
	}

	svc := NewGameService(mock)
	err := svc.DeleteGame(context.Background(), 1)

	require.Error(t, err)
}
