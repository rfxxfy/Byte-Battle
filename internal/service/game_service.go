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
	ErrNotEnoughPlayers     = errors.New("at least two players are required")
	ErrDuplicatePlayers     = errors.New("players must be different")
	ErrGameNotFound         = errors.New("game not found")
	ErrGameAlreadyStarted   = errors.New("game already started or completed")
	ErrGameNotInProgress    = errors.New("game is not in progress")
	ErrInvalidWinner        = errors.New("winner must be one of the players")
	ErrCannotCancelFinished = errors.New("cannot cancel finished game")
	ErrGameAlreadyCancelled = errors.New("game is already cancelled")
)

type GameService struct {
	repo database.IGameRepo
}

func NewGameService(repo database.IGameRepo) *GameService {
	return &GameService{repo: repo}
}

func (s *GameService) CreateGame(ctx context.Context, playerIDs []int, problemID int) (*models.Game, error) {
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

func (s *GameService) getGame(ctx context.Context, id int) (*models.Game, error) {
	game, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return nil, ErrGameNotFound
		}
		return nil, err
	}
	return game, nil
}

func (s *GameService) GetGame(ctx context.Context, id int) (*models.Game, error) {
	return s.getGame(ctx, id)
}

func (s *GameService) ListGames(ctx context.Context, limit, offset int) (models.GameSlice, error) {
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	return s.repo.GetAll(ctx, limit, offset)
}

func (s *GameService) StartGame(ctx context.Context, id int) (*models.Game, error) {
	game, err := s.getGame(ctx, id)
	if err != nil {
		return nil, err
	}

	if game.Status != database.GameStatusPending {
		return nil, ErrGameAlreadyStarted
	}

	game.Status = database.GameStatusActive
	game.StartedAt = null.TimeFrom(time.Now())

	err = s.repo.Upsert(ctx, game)
	if err != nil {
		return nil, err
	}

	return game, nil
}

func (s *GameService) CompleteGame(ctx context.Context, id, winnerID int) (*models.Game, error) {
	game, err := s.getGame(ctx, id)
	if err != nil {
		return nil, err
	}

	if game.Status != database.GameStatusActive {
		return nil, ErrGameNotInProgress
	}

	ok, err := s.repo.IsParticipant(ctx, id, winnerID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrInvalidWinner
	}

	game.Status = database.GameStatusFinished
	game.WinnerID = null.IntFrom(winnerID)
	game.CompletedAt = null.TimeFrom(time.Now())

	err = s.repo.Upsert(ctx, game)
	if err != nil {
		return nil, err
	}

	return game, nil
}

func (s *GameService) CancelGame(ctx context.Context, id int) (*models.Game, error) {
	game, err := s.getGame(ctx, id)
	if err != nil {
		return nil, err
	}

	if game.Status == database.GameStatusFinished {
		return nil, ErrCannotCancelFinished
	}
	if game.Status == database.GameStatusCancelled {
		return nil, ErrGameAlreadyCancelled
	}

	game.Status = database.GameStatusCancelled

	err = s.repo.Upsert(ctx, game)
	if err != nil {
		return nil, err
	}

	return game, nil
}

func (s *GameService) DeleteGame(ctx context.Context, id int) error {
	err := s.repo.Delete(ctx, id)
	if errors.Is(err, database.ErrNotFound) {
		return ErrGameNotFound
	}
	return err
}
