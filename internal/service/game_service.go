package service

import (
	"context"
	"errors"

	"bytebattle/internal/apierr"
	sqlcdb "bytebattle/internal/db/sqlc"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	gameStatusPending   = "pending"
	gameStatusActive    = "active"
	gameStatusFinished  = "finished"
	gameStatusCancelled = "cancelled"
)

type GameService struct {
	q    *sqlcdb.Queries
	pool *pgxpool.Pool
}

func NewGameService(q *sqlcdb.Queries, pool *pgxpool.Pool) *GameService {
	return &GameService{q: q, pool: pool}
}

func (s *GameService) CreateGame(ctx context.Context, playerIDs []int, problemID int) (sqlcdb.Game, error) {
	if len(playerIDs) < 2 {
		return sqlcdb.Game{}, apierr.New(apierr.ErrNotEnoughPlayers, "at least two players are required")
	}
	seen := make(map[int]struct{})
	for _, id := range playerIDs {
		if _, exists := seen[id]; exists {
			return sqlcdb.Game{}, apierr.New(apierr.ErrDuplicatePlayers, "players must be different")
		}
		seen[id] = struct{}{}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return sqlcdb.Game{}, err
	}
	defer tx.Rollback(ctx)

	qtx := s.q.WithTx(tx)

	game, err := qtx.CreateGame(ctx, int32(problemID))
	if err != nil {
		return sqlcdb.Game{}, err
	}

	for _, pid := range playerIDs {
		if err := qtx.AddGameParticipant(ctx, sqlcdb.AddGameParticipantParams{
			GameID: game.ID,
			UserID: int32(pid),
		}); err != nil {
			return sqlcdb.Game{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return sqlcdb.Game{}, err
	}

	return game, nil
}

func (s *GameService) GetGame(ctx context.Context, id int) (sqlcdb.Game, error) {
	game, err := s.q.GetGameByID(ctx, int32(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlcdb.Game{}, apierr.New(apierr.ErrGameNotFound, "game not found")
	}
	return game, err
}

func (s *GameService) ListGames(ctx context.Context, limit, offset int) ([]sqlcdb.Game, int64, error) {
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	total, err := s.q.CountGames(ctx)
	if err != nil {
		return nil, 0, err
	}

	games, err := s.q.ListGames(ctx, sqlcdb.ListGamesParams{
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, 0, err
	}

	return games, total, nil
}

func (s *GameService) StartGame(ctx context.Context, id int) (sqlcdb.Game, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return sqlcdb.Game{}, err
	}
	defer tx.Rollback(ctx)

	qtx := s.q.WithTx(tx)

	game, err := qtx.GetGameForUpdate(ctx, int32(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlcdb.Game{}, apierr.New(apierr.ErrGameNotFound, "game not found")
	}
	if err != nil {
		return sqlcdb.Game{}, err
	}

	if game.Status != gameStatusPending {
		return sqlcdb.Game{}, apierr.New(apierr.ErrGameAlreadyStarted, "game already started or completed")
	}

	game, err = qtx.StartGame(ctx, game.ID)
	if err != nil {
		return sqlcdb.Game{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return sqlcdb.Game{}, err
	}

	return game, nil
}

func (s *GameService) CompleteGame(ctx context.Context, id, winnerID int) (sqlcdb.Game, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return sqlcdb.Game{}, err
	}
	defer tx.Rollback(ctx)

	qtx := s.q.WithTx(tx)

	game, err := qtx.GetGameForUpdate(ctx, int32(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlcdb.Game{}, apierr.New(apierr.ErrGameNotFound, "game not found")
	}
	if err != nil {
		return sqlcdb.Game{}, err
	}

	if game.Status != gameStatusActive {
		return sqlcdb.Game{}, apierr.New(apierr.ErrGameNotInProgress, "game is not in progress")
	}

	ok, err := qtx.IsGameParticipant(ctx, sqlcdb.IsGameParticipantParams{
		GameID: game.ID,
		UserID: int32(winnerID),
	})
	if err != nil {
		return sqlcdb.Game{}, err
	}
	if !ok {
		return sqlcdb.Game{}, apierr.New(apierr.ErrInvalidWinner, "winner must be one of the players")
	}

	game, err = qtx.CompleteGame(ctx, sqlcdb.CompleteGameParams{
		ID:       game.ID,
		WinnerID: pgtype.Int4{Int32: int32(winnerID), Valid: true},
	})
	if err != nil {
		return sqlcdb.Game{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return sqlcdb.Game{}, err
	}

	return game, nil
}

func (s *GameService) CancelGame(ctx context.Context, id int) (sqlcdb.Game, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return sqlcdb.Game{}, err
	}
	defer tx.Rollback(ctx)

	qtx := s.q.WithTx(tx)

	game, err := qtx.GetGameForUpdate(ctx, int32(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlcdb.Game{}, apierr.New(apierr.ErrGameNotFound, "game not found")
	}
	if err != nil {
		return sqlcdb.Game{}, err
	}

	if game.Status == gameStatusFinished {
		return sqlcdb.Game{}, apierr.New(apierr.ErrCannotCancelFinishedGame, "cannot cancel finished game")
	}
	if game.Status == gameStatusCancelled {
		return sqlcdb.Game{}, apierr.New(apierr.ErrGameAlreadyCancelled, "game is already cancelled")
	}

	game, err = qtx.CancelGame(ctx, game.ID)
	if err != nil {
		return sqlcdb.Game{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return sqlcdb.Game{}, err
	}

	return game, nil
}

func (s *GameService) DeleteGame(ctx context.Context, id int) error {
	rowsAff, err := s.q.DeleteGame(ctx, int32(id))
	if err != nil {
		return err
	}
	if rowsAff == 0 {
		return apierr.New(apierr.ErrGameNotFound, "game not found")
	}
	return nil
}
