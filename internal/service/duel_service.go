package service

import (
	"context"
	"errors"
	"time"

	"bytebattle/internal/database"
	"bytebattle/internal/database/models"

	"github.com/aarondl/null/v8"
)

type DuelService struct {
	repo *database.DuelRepository
}

func NewDuelService(repo *database.DuelRepository) *DuelService {
	return &DuelService{repo: repo}
}

func (s *DuelService) CreateDuel(ctx context.Context, player1ID, player2ID, problemID int) (*models.Duel, error) {
	if player1ID == player2ID {
		return nil, errors.New("players must be different")
	}

	return s.repo.Create(ctx, player1ID, player2ID, problemID)
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

	if duel.Status != "pending" {
		return nil, errors.New("duel already started or completed")
	}

	duel.Status = "in_progress"
	duel.StartedAt = null.TimeFrom(time.Now())

	err = s.repo.Update(ctx, duel)
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

	if duel.Status != "in_progress" {
		return nil, errors.New("duel is not in progress")
	}

	if winnerID != duel.Player1ID && winnerID != duel.Player2ID {
		return nil, errors.New("winner must be one of the players")
	}

	duel.Status = "completed"
	duel.WinnerID = null.IntFrom(winnerID)
	duel.CompletedAt = null.TimeFrom(time.Now())

	err = s.repo.Update(ctx, duel)
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

	if duel.Status == "completed" {
		return nil, errors.New("cannot cancel completed duel")
	}

	duel.Status = "cancelled"

	err = s.repo.Update(ctx, duel)
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
	return s.repo.GetByStatus(ctx, "in_progress")
}
