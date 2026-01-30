package service

import (
	"context"
	"errors"
	"time"

	"bytebattle/internal/database"
	"bytebattle/internal/database/models"

	"github.com/aarondl/null/v8"
)

var (
	ErrNotEnoughPlayers      = errors.New("at least two players are required")
	ErrDuplicatePlayers      = errors.New("players must be different")
	ErrDuelAlreadyStarted    = errors.New("duel already started or completed")
	ErrDuelNotInProgress     = errors.New("duel is not in progress")
	ErrInvalidWinner         = errors.New("winner must be one of the players")
	ErrCannotCancelCompleted = errors.New("cannot cancel completed duel")
)

type DuelService struct {
	repo database.IDuelRepo
}

func NewDuelService(repo database.IDuelRepo) *DuelService {
	return &DuelService{repo: repo}
}

func NewDuelServiceWithRepo(repo database.IDuelRepo) *DuelService {
	return &DuelService{repo: repo}
}

func (s *DuelService) CreateDuel(ctx context.Context, playerIDs []int, problemID int) (*models.Duel, error) {
	if len(playerIDs) < 2 {
		return nil, ErrNotEnoughPlayers
	}

	seen := make(map[int]struct{})
	for _, id := range playerIDs {
		if _, exists := seen[id]; exists {
			return nil, ErrDuplicatePlayers
		}
		seen[id] = struct{}{}
	}

	players := make([]database.Player, len(playerIDs))
	for i, id := range playerIDs {
		players[i] = database.Player{ID: id}
	}

	return s.repo.Create(ctx, players, problemID)
}

func (s *DuelService) GetDuel(ctx context.Context, id int) (*models.Duel, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *DuelService) ListDuels(ctx context.Context, limit, offset int) (models.DuelSlice, error) {
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}

	return s.repo.GetAll(ctx, limit, offset)
}

func (s *DuelService) StartDuel(ctx context.Context, id int) (*models.Duel, error) {
	duel, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if duel.Status != string(database.DuelStatusPending) {
		return nil, ErrDuelAlreadyStarted
	}

	duel.Status = string(database.DuelStatusActive)
	duel.StartedAt = null.TimeFrom(time.Now())

	err = s.repo.Upsert(ctx, duel)
	if err != nil {
		return nil, err
	}

	return duel, nil
}

func (s *DuelService) CompleteDuel(ctx context.Context, id int, winnerID int) (*models.Duel, error) {
	duel, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if duel.Status != string(database.DuelStatusActive) {
		return nil, ErrDuelNotInProgress
	}

	if winnerID != duel.Player1ID && winnerID != duel.Player2ID {
		return nil, ErrInvalidWinner
	}

	duel.Status = string(database.DuelStatusFinished)
	duel.WinnerID = null.IntFrom(winnerID)
	duel.CompletedAt = null.TimeFrom(time.Now())

	err = s.repo.Upsert(ctx, duel)
	if err != nil {
		return nil, err
	}

	return duel, nil
}

func (s *DuelService) CancelDuel(ctx context.Context, id int) (*models.Duel, error) {
	duel, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if duel.Status == string(database.DuelStatusFinished) {
		return nil, ErrCannotCancelCompleted
	}

	duel.Status = "cancelled"

	err = s.repo.Upsert(ctx, duel)
	if err != nil {
		return nil, err
	}

	return duel, nil
}

func (s *DuelService) DeleteDuel(ctx context.Context, id int) error {
	return s.repo.Delete(ctx, id)
}

func (s *DuelService) GetPlayerDuels(ctx context.Context, playerID int) (models.DuelSlice, error) {
	return s.repo.GetByPlayerID(ctx, playerID)
}

func (s *DuelService) GetActiveDuels(ctx context.Context) (models.DuelSlice, error) {
	return s.repo.GetByStatus(ctx, database.DuelStatusActive)
}
