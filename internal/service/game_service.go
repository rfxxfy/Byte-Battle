package service

import (
	"context"
	"errors"

	"bytebattle/internal/database"
	"bytebattle/internal/database/models"
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

func (s *GameService) GetGame(ctx context.Context, id int) (*models.Game, error) {
	game, err := s.repo.GetByID(ctx, id)
	if errors.Is(err, database.ErrNotFound) {
		return nil, ErrGameNotFound
	}
	return game, err
}

func (s *GameService) ListGames(ctx context.Context, limit, offset int) (models.GameSlice, int64, error) {
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	total, err := s.repo.Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	games, err := s.repo.GetAll(ctx, limit, offset)
	if err != nil {
		return nil, 0, err
	}

	return games, total, nil
}

func (s *GameService) StartGame(ctx context.Context, id int) (*models.Game, error) {
	game, err := s.repo.StartGame(ctx, id)
	if err != nil {
		switch {
		case errors.Is(err, database.ErrNotFound):
			return nil, ErrGameNotFound
		case errors.Is(err, database.ErrGameNotPending):
			return nil, ErrGameAlreadyStarted
		}
		return nil, err
	}
	return game, nil
}

func (s *GameService) CompleteGame(ctx context.Context, id, winnerID int) (*models.Game, error) {
	game, err := s.repo.CompleteGame(ctx, id, winnerID)
	if err != nil {
		switch {
		case errors.Is(err, database.ErrNotFound):
			return nil, ErrGameNotFound
		case errors.Is(err, database.ErrGameNotActive):
			return nil, ErrGameNotInProgress
		case errors.Is(err, database.ErrNotParticipant):
			return nil, ErrInvalidWinner
		}
		return nil, err
	}
	return game, nil
}

func (s *GameService) CancelGame(ctx context.Context, id int) (*models.Game, error) {
	game, err := s.repo.CancelGame(ctx, id)
	if err != nil {
		switch {
		case errors.Is(err, database.ErrNotFound):
			return nil, ErrGameNotFound
		case errors.Is(err, database.ErrGameFinished):
			return nil, ErrCannotCancelFinished
		case errors.Is(err, database.ErrGameAlreadyCancelled):
			return nil, ErrGameAlreadyCancelled
		}
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
