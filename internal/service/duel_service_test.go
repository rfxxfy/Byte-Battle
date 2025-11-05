package service

import (
	"context"
	"errors"
	"testing"

	"bytebattle/internal/database"
	"bytebattle/internal/database/models"
)

type mockDuelRepository struct {
	createFunc        func(ctx context.Context, players []database.Player, problemID int) (*models.Duel, error)
	getByIDFunc       func(ctx context.Context, id int) (*models.Duel, error)
	getAllFunc        func(ctx context.Context, limit, offset int) (models.DuelSlice, error)
	upsertFunc        func(ctx context.Context, duel *models.Duel) error
	deleteFunc        func(ctx context.Context, id int) error
	getByPlayerIDFunc func(ctx context.Context, playerID int) (models.DuelSlice, error)
	getByStatusFunc   func(ctx context.Context, status database.DuelStatus) (models.DuelSlice, error)
}

func (m *mockDuelRepository) Create(ctx context.Context, players []database.Player, problemID int) (*models.Duel, error) {
	if m.createFunc != nil {
		return m.createFunc(ctx, players, problemID)
	}
	return &models.Duel{ID: 1}, nil
}

func (m *mockDuelRepository) GetByID(ctx context.Context, id int) (*models.Duel, error) {
	if m.getByIDFunc != nil {
		return m.getByIDFunc(ctx, id)
	}
	return &models.Duel{ID: id, Status: string(database.DuelStatusPending)}, nil
}

func (m *mockDuelRepository) GetAll(ctx context.Context, limit, offset int) (models.DuelSlice, error) {
	if m.getAllFunc != nil {
		return m.getAllFunc(ctx, limit, offset)
	}
	return models.DuelSlice{}, nil
}

func (m *mockDuelRepository) Upsert(ctx context.Context, duel *models.Duel) error {
	if m.upsertFunc != nil {
		return m.upsertFunc(ctx, duel)
	}
	return nil
}

func (m *mockDuelRepository) Delete(ctx context.Context, id int) error {
	if m.deleteFunc != nil {
		return m.deleteFunc(ctx, id)
	}
	return nil
}

func (m *mockDuelRepository) GetByPlayerID(ctx context.Context, playerID int) (models.DuelSlice, error) {
	if m.getByPlayerIDFunc != nil {
		return m.getByPlayerIDFunc(ctx, playerID)
	}
	return models.DuelSlice{}, nil
}

func (m *mockDuelRepository) GetByStatus(ctx context.Context, status database.DuelStatus) (models.DuelSlice, error) {
	if m.getByStatusFunc != nil {
		return m.getByStatusFunc(ctx, status)
	}
	return models.DuelSlice{}, nil
}

func TestCreateDuel_Success(t *testing.T) {
	mock := &mockDuelRepository{
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

	svc := NewDuelServiceWithRepo(mock)
	duel, err := svc.CreateDuel(context.Background(), []int{1, 2}, 10)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if duel.ID != 1 {
		t.Errorf("expected duel ID 1, got %d", duel.ID)
	}
	if duel.Player1ID != 1 {
		t.Errorf("expected player1 ID 1, got %d", duel.Player1ID)
	}
	if duel.Player2ID != 2 {
		t.Errorf("expected player2 ID 2, got %d", duel.Player2ID)
	}
}

func TestCreateDuel_ThreePlayers(t *testing.T) {
	var capturedPlayers []database.Player
	mock := &mockDuelRepository{
		createFunc: func(ctx context.Context, players []database.Player, problemID int) (*models.Duel, error) {
			capturedPlayers = players
			return &models.Duel{ID: 1}, nil
		},
	}

	svc := NewDuelServiceWithRepo(mock)
	_, err := svc.CreateDuel(context.Background(), []int{1, 2, 3}, 10)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(capturedPlayers) != 3 {
		t.Errorf("expected 3 players, got %d", len(capturedPlayers))
	}
}

func TestCreateDuel_NotEnoughPlayers(t *testing.T) {
	mock := &mockDuelRepository{}
	svc := NewDuelServiceWithRepo(mock)

	_, err := svc.CreateDuel(context.Background(), []int{1}, 10)

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "at least two players are required" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestCreateDuel_EmptyPlayers(t *testing.T) {
	mock := &mockDuelRepository{}
	svc := NewDuelServiceWithRepo(mock)

	_, err := svc.CreateDuel(context.Background(), []int{}, 10)

	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestCreateDuel_DuplicatePlayers(t *testing.T) {
	mock := &mockDuelRepository{}
	svc := NewDuelServiceWithRepo(mock)

	_, err := svc.CreateDuel(context.Background(), []int{1, 1}, 10)

	if err == nil {
		t.Fatal("expected error for duplicate players, got nil")
	}
	if err.Error() != "players must be different" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestCreateDuel_DuplicateInThree(t *testing.T) {
	mock := &mockDuelRepository{}
	svc := NewDuelServiceWithRepo(mock)

	_, err := svc.CreateDuel(context.Background(), []int{1, 2, 1}, 10)

	if err == nil {
		t.Fatal("expected error for duplicate players, got nil")
	}
}

func TestCreateDuel_RepoError(t *testing.T) {
	mock := &mockDuelRepository{
		createFunc: func(ctx context.Context, players []database.Player, problemID int) (*models.Duel, error) {
			return nil, errors.New("database error")
		},
	}

	svc := NewDuelServiceWithRepo(mock)
	_, err := svc.CreateDuel(context.Background(), []int{1, 2}, 10)

	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGetDuel_Success(t *testing.T) {
	mock := &mockDuelRepository{
		getByIDFunc: func(ctx context.Context, id int) (*models.Duel, error) {
			return &models.Duel{ID: id}, nil
		},
	}

	svc := NewDuelServiceWithRepo(mock)
	duel, err := svc.GetDuel(context.Background(), 5)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if duel.ID != 5 {
		t.Errorf("expected ID 5, got %d", duel.ID)
	}
}

func TestGetDuel_NotFound(t *testing.T) {
	mock := &mockDuelRepository{
		getByIDFunc: func(ctx context.Context, id int) (*models.Duel, error) {
			return nil, errors.New("not found")
		},
	}

	svc := NewDuelServiceWithRepo(mock)
	_, err := svc.GetDuel(context.Background(), 999)

	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestListDuels_DefaultLimit(t *testing.T) {
	var capturedLimit int
	mock := &mockDuelRepository{
		getAllFunc: func(ctx context.Context, limit, offset int) (models.DuelSlice, error) {
			capturedLimit = limit
			return models.DuelSlice{}, nil
		},
	}

	svc := NewDuelServiceWithRepo(mock)
	_, _ = svc.ListDuels(context.Background(), 0, 0)

	if capturedLimit != 10 {
		t.Errorf("expected default limit 10, got %d", capturedLimit)
	}
}

func TestListDuels_NegativeLimit(t *testing.T) {
	var capturedLimit int
	mock := &mockDuelRepository{
		getAllFunc: func(ctx context.Context, limit, offset int) (models.DuelSlice, error) {
			capturedLimit = limit
			return models.DuelSlice{}, nil
		},
	}

	svc := NewDuelServiceWithRepo(mock)
	_, _ = svc.ListDuels(context.Background(), -5, 0)

	if capturedLimit != 10 {
		t.Errorf("expected default limit 10, got %d", capturedLimit)
	}
}

func TestListDuels_MaxLimit(t *testing.T) {
	var capturedLimit int
	mock := &mockDuelRepository{
		getAllFunc: func(ctx context.Context, limit, offset int) (models.DuelSlice, error) {
			capturedLimit = limit
			return models.DuelSlice{}, nil
		},
	}

	svc := NewDuelServiceWithRepo(mock)
	_, _ = svc.ListDuels(context.Background(), 500, 0)

	if capturedLimit != 100 {
		t.Errorf("expected max limit 100, got %d", capturedLimit)
	}
}

func TestListDuels_ValidLimit(t *testing.T) {
	var capturedLimit int
	mock := &mockDuelRepository{
		getAllFunc: func(ctx context.Context, limit, offset int) (models.DuelSlice, error) {
			capturedLimit = limit
			return models.DuelSlice{}, nil
		},
	}

	svc := NewDuelServiceWithRepo(mock)
	_, _ = svc.ListDuels(context.Background(), 50, 0)

	if capturedLimit != 50 {
		t.Errorf("expected limit 50, got %d", capturedLimit)
	}
}

func TestStartDuel_Success(t *testing.T) {
	mock := &mockDuelRepository{
		getByIDFunc: func(ctx context.Context, id int) (*models.Duel, error) {
			return &models.Duel{ID: id, Status: string(database.DuelStatusPending)}, nil
		},
		upsertFunc: func(ctx context.Context, duel *models.Duel) error {
			return nil
		},
	}

	svc := NewDuelServiceWithRepo(mock)
	duel, err := svc.StartDuel(context.Background(), 1)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if duel.Status != string(database.DuelStatusActive) {
		t.Errorf("expected status active, got %s", duel.Status)
	}
}

func TestStartDuel_AlreadyActive(t *testing.T) {
	mock := &mockDuelRepository{
		getByIDFunc: func(ctx context.Context, id int) (*models.Duel, error) {
			return &models.Duel{ID: id, Status: string(database.DuelStatusActive)}, nil
		},
	}

	svc := NewDuelServiceWithRepo(mock)
	_, err := svc.StartDuel(context.Background(), 1)

	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestStartDuel_AlreadyFinished(t *testing.T) {
	mock := &mockDuelRepository{
		getByIDFunc: func(ctx context.Context, id int) (*models.Duel, error) {
			return &models.Duel{ID: id, Status: string(database.DuelStatusFinished)}, nil
		},
	}

	svc := NewDuelServiceWithRepo(mock)
	_, err := svc.StartDuel(context.Background(), 1)

	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestStartDuel_NotFound(t *testing.T) {
	mock := &mockDuelRepository{
		getByIDFunc: func(ctx context.Context, id int) (*models.Duel, error) {
			return nil, errors.New("not found")
		},
	}

	svc := NewDuelServiceWithRepo(mock)
	_, err := svc.StartDuel(context.Background(), 1)

	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestStartDuel_UpsertError(t *testing.T) {
	mock := &mockDuelRepository{
		getByIDFunc: func(ctx context.Context, id int) (*models.Duel, error) {
			return &models.Duel{ID: id, Status: string(database.DuelStatusPending)}, nil
		},
		upsertFunc: func(ctx context.Context, duel *models.Duel) error {
			return errors.New("upsert error")
		},
	}

	svc := NewDuelServiceWithRepo(mock)
	_, err := svc.StartDuel(context.Background(), 1)

	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestCompleteDuel_Success(t *testing.T) {
	mock := &mockDuelRepository{
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

	svc := NewDuelServiceWithRepo(mock)
	duel, err := svc.CompleteDuel(context.Background(), 1, 1)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if duel.Status != string(database.DuelStatusFinished) {
		t.Errorf("expected status finished, got %s", duel.Status)
	}
	if !duel.WinnerID.Valid || duel.WinnerID.Int != 1 {
		t.Errorf("expected winner ID 1, got %v", duel.WinnerID)
	}
}

func TestCompleteDuel_Player2Wins(t *testing.T) {
	mock := &mockDuelRepository{
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

	svc := NewDuelServiceWithRepo(mock)
	duel, err := svc.CompleteDuel(context.Background(), 1, 2)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !duel.WinnerID.Valid || duel.WinnerID.Int != 2 {
		t.Errorf("expected winner ID 2, got %v", duel.WinnerID)
	}
}

func TestCompleteDuel_InvalidWinner(t *testing.T) {
	mock := &mockDuelRepository{
		getByIDFunc: func(ctx context.Context, id int) (*models.Duel, error) {
			return &models.Duel{
				ID:        id,
				Status:    string(database.DuelStatusActive),
				Player1ID: 1,
				Player2ID: 2,
			}, nil
		},
	}

	svc := NewDuelServiceWithRepo(mock)
	_, err := svc.CompleteDuel(context.Background(), 1, 999)

	if err == nil {
		t.Fatal("expected error for invalid winner, got nil")
	}
	if err.Error() != "winner must be one of the players" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestCompleteDuel_NotInProgress(t *testing.T) {
	mock := &mockDuelRepository{
		getByIDFunc: func(ctx context.Context, id int) (*models.Duel, error) {
			return &models.Duel{ID: id, Status: string(database.DuelStatusPending)}, nil
		},
	}

	svc := NewDuelServiceWithRepo(mock)
	_, err := svc.CompleteDuel(context.Background(), 1, 1)

	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestCancelDuel_Success(t *testing.T) {
	mock := &mockDuelRepository{
		getByIDFunc: func(ctx context.Context, id int) (*models.Duel, error) {
			return &models.Duel{ID: id, Status: string(database.DuelStatusPending)}, nil
		},
		upsertFunc: func(ctx context.Context, duel *models.Duel) error {
			return nil
		},
	}

	svc := NewDuelServiceWithRepo(mock)
	duel, err := svc.CancelDuel(context.Background(), 1)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if duel.Status != "cancelled" {
		t.Errorf("expected status cancelled, got %s", duel.Status)
	}
}

func TestCancelDuel_ActiveDuel(t *testing.T) {
	mock := &mockDuelRepository{
		getByIDFunc: func(ctx context.Context, id int) (*models.Duel, error) {
			return &models.Duel{ID: id, Status: string(database.DuelStatusActive)}, nil
		},
		upsertFunc: func(ctx context.Context, duel *models.Duel) error {
			return nil
		},
	}

	svc := NewDuelServiceWithRepo(mock)
	duel, err := svc.CancelDuel(context.Background(), 1)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if duel.Status != "cancelled" {
		t.Errorf("expected status cancelled, got %s", duel.Status)
	}
}

func TestCancelDuel_AlreadyCompleted(t *testing.T) {
	mock := &mockDuelRepository{
		getByIDFunc: func(ctx context.Context, id int) (*models.Duel, error) {
			return &models.Duel{ID: id, Status: string(database.DuelStatusFinished)}, nil
		},
	}

	svc := NewDuelServiceWithRepo(mock)
	_, err := svc.CancelDuel(context.Background(), 1)

	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestDeleteDuel_Success(t *testing.T) {
	mock := &mockDuelRepository{
		deleteFunc: func(ctx context.Context, id int) error {
			return nil
		},
	}

	svc := NewDuelServiceWithRepo(mock)
	err := svc.DeleteDuel(context.Background(), 1)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteDuel_Error(t *testing.T) {
	mock := &mockDuelRepository{
		deleteFunc: func(ctx context.Context, id int) error {
			return errors.New("delete error")
		},
	}

	svc := NewDuelServiceWithRepo(mock)
	err := svc.DeleteDuel(context.Background(), 1)

	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGetPlayerDuels_Success(t *testing.T) {
	mock := &mockDuelRepository{
		getByPlayerIDFunc: func(ctx context.Context, playerID int) (models.DuelSlice, error) {
			return models.DuelSlice{&models.Duel{ID: 1}, &models.Duel{ID: 2}}, nil
		},
	}

	svc := NewDuelServiceWithRepo(mock)
	duels, err := svc.GetPlayerDuels(context.Background(), 1)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(duels) != 2 {
		t.Errorf("expected 2 duels, got %d", len(duels))
	}
}

func TestGetActiveDuels_Success(t *testing.T) {
	var capturedStatus database.DuelStatus
	mock := &mockDuelRepository{
		getByStatusFunc: func(ctx context.Context, status database.DuelStatus) (models.DuelSlice, error) {
			capturedStatus = status
			return models.DuelSlice{&models.Duel{ID: 1}}, nil
		},
	}

	svc := NewDuelServiceWithRepo(mock)
	duels, err := svc.GetActiveDuels(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(duels) != 1 {
		t.Errorf("expected 1 duel, got %d", len(duels))
	}
	if capturedStatus != database.DuelStatusActive {
		t.Errorf("expected status active, got %s", capturedStatus)
	}
}
