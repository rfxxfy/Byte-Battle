package service

import (
	"context"
	"errors"

	"bytebattle/internal/apierr"
	"bytebattle/internal/database"
	"bytebattle/internal/database/models"
)

type GameService struct {
	repo database.IGameRepo
}

func NewGameService(repo database.IGameRepo) *GameService {
	return &GameService{repo: repo}
}

func (s *GameService) CreateGame(ctx context.Context, playerIDs []int, problemID int) (*models.Game, error) {
	if len(playerIDs) < 2 {
		return nil, apierr.New(apierr.ErrNotEnoughPlayers, "at least two players are required")
	}
	seen := make(map[int]struct{})
	for _, id := range playerIDs {
		if _, exists := seen[id]; exists {
			return nil, apierr.New(apierr.ErrDuplicatePlayers, "players must be different")
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
		return nil, apierr.New(apierr.ErrGameNotFound, "game not found")
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
			return nil, apierr.New(apierr.ErrGameNotFound, "game not found")
		case errors.Is(err, database.ErrGameNotPending):
			return nil, apierr.New(apierr.ErrGameAlreadyStarted, "game already started or completed")
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
			return nil, apierr.New(apierr.ErrGameNotFound, "game not found")
		case errors.Is(err, database.ErrGameNotActive):
			return nil, apierr.New(apierr.ErrGameNotInProgress, "game is not in progress")
		case errors.Is(err, database.ErrNotParticipant):
			return nil, apierr.New(apierr.ErrInvalidWinner, "winner must be one of the players")
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
			return nil, apierr.New(apierr.ErrGameNotFound, "game not found")
		case errors.Is(err, database.ErrGameFinished):
			return nil, apierr.New(apierr.ErrCannotCancelFinished, "cannot cancel finished game")
		case errors.Is(err, database.ErrGameAlreadyCancelled):
			return nil, apierr.New(apierr.ErrGameAlreadyCancelled, "game is already cancelled")
		}
		return nil, err
	}
	return game, nil
}

func (s *GameService) DeleteGame(ctx context.Context, id int) error {
	err := s.repo.Delete(ctx, id)
	if errors.Is(err, database.ErrNotFound) {
		return apierr.New(apierr.ErrGameNotFound, "game not found")
	}
	return err
}
