package service

import (
	"context"
	"errors"

	"bytebattle/internal/apierr"
	sqlcdb "bytebattle/internal/db/sqlc"
	"bytebattle/internal/problems"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Participant struct {
	ID   uuid.UUID
	Name *string
}

const (
	gameStatusPending   = "pending"
	gameStatusActive    = "active"
	gameStatusFinished  = "finished"
	gameStatusCancelled = "cancelled"
)

type GameService struct {
	q        *sqlcdb.Queries
	pool     *pgxpool.Pool
	problems *problems.Loader
}

func NewGameService(q *sqlcdb.Queries, pool *pgxpool.Pool, loader *problems.Loader) *GameService {
	return &GameService{q: q, pool: pool, problems: loader}
}

func (s *GameService) CreateGame(ctx context.Context, creatorID uuid.UUID, problemID string) (sqlcdb.Game, error) {
	if _, err := s.problems.Get(problemID); err != nil {
		return sqlcdb.Game{}, apierr.New(apierr.ErrProblemNotFound, "problem not found")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return sqlcdb.Game{}, err
	}
	defer tx.Rollback(ctx)

	qtx := s.q.WithTx(tx)

	game, err := qtx.CreateGame(ctx, sqlcdb.CreateGameParams{
		ProblemID: problemID,
		CreatorID: creatorID,
	})
	if err != nil {
		return sqlcdb.Game{}, err
	}

	if err := qtx.AddGameParticipant(ctx, sqlcdb.AddGameParticipantParams{
		GameID: game.ID,
		UserID: creatorID,
	}); err != nil {
		return sqlcdb.Game{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return sqlcdb.Game{}, err
	}

	return game, nil
}

func (s *GameService) JoinGame(ctx context.Context, gameID int, userID uuid.UUID) (sqlcdb.Game, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return sqlcdb.Game{}, err
	}
	defer tx.Rollback(ctx)

	qtx := s.q.WithTx(tx)

	game, err := qtx.GetGameForUpdate(ctx, int32(gameID))
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlcdb.Game{}, apierr.New(apierr.ErrGameNotFound, "game not found")
	}
	if err != nil {
		return sqlcdb.Game{}, err
	}

	if game.Status != gameStatusPending {
		return sqlcdb.Game{}, apierr.New(apierr.ErrGameAlreadyStarted, "game is not pending")
	}

	already, err := qtx.IsGameParticipant(ctx, sqlcdb.IsGameParticipantParams{
		GameID: game.ID,
		UserID: userID,
	})
	if err != nil {
		return sqlcdb.Game{}, err
	}
	if already {
		return sqlcdb.Game{}, apierr.New(apierr.ErrAlreadyParticipant, "already a participant")
	}

	if err := qtx.AddGameParticipant(ctx, sqlcdb.AddGameParticipantParams{
		GameID: game.ID,
		UserID: userID,
	}); err != nil {
		return sqlcdb.Game{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return sqlcdb.Game{}, err
	}

	return game, nil
}

func (s *GameService) GetParticipants(ctx context.Context, gameID int) ([]Participant, error) {
	rows, err := s.q.GetParticipants(ctx, int32(gameID))
	if err != nil {
		return nil, err
	}
	result := make([]Participant, len(rows))
	for i, r := range rows {
		p := Participant{ID: r.UserID}
		if r.Name.Valid {
			p.Name = &r.Name.String
		}
		result[i] = p
	}
	return result, nil
}

func (s *GameService) GetParticipantsByGameIDs(ctx context.Context, gameIDs []int32) (map[int32][]Participant, error) {
	rows, err := s.q.GetParticipantsByGameIDs(ctx, gameIDs)
	if err != nil {
		return nil, err
	}
	result := make(map[int32][]Participant, len(gameIDs))
	for _, r := range rows {
		p := Participant{ID: r.UserID}
		if r.Name.Valid {
			p.Name = &r.Name.String
		}
		result[r.GameID] = append(result[r.GameID], p)
	}
	return result, nil
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

func (s *GameService) StartGame(ctx context.Context, id int, userID uuid.UUID) (sqlcdb.Game, error) {
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

	if game.CreatorID != userID {
		return sqlcdb.Game{}, apierr.New(apierr.ErrNotGameCreator, "only the game creator can start the game")
	}

	count, err := qtx.CountGameParticipants(ctx, game.ID)
	if err != nil {
		return sqlcdb.Game{}, err
	}
	if count < 2 {
		return sqlcdb.Game{}, apierr.New(apierr.ErrNotEnoughPlayers, "at least two players must join before starting")
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

func (s *GameService) CompleteGame(ctx context.Context, id int, winnerID uuid.UUID) (sqlcdb.Game, error) {
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
		UserID: winnerID,
	})
	if err != nil {
		return sqlcdb.Game{}, err
	}
	if !ok {
		return sqlcdb.Game{}, apierr.New(apierr.ErrInvalidWinner, "winner must be one of the players")
	}

	game, err = qtx.CompleteGame(ctx, sqlcdb.CompleteGameParams{
		ID:       game.ID,
		WinnerID: uuid.NullUUID{UUID: winnerID, Valid: true},
	})
	if err != nil {
		return sqlcdb.Game{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return sqlcdb.Game{}, err
	}

	return game, nil
}

func (s *GameService) CancelGame(ctx context.Context, id int, userID uuid.UUID) (sqlcdb.Game, error) {
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

	if game.CreatorID != userID {
		return sqlcdb.Game{}, apierr.New(apierr.ErrNotGameCreator, "only the game creator can cancel the game")
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

func (s *GameService) LeaveGame(ctx context.Context, gameID int, userID uuid.UUID) (sqlcdb.Game, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return sqlcdb.Game{}, err
	}
	defer tx.Rollback(ctx)

	qtx := s.q.WithTx(tx)

	game, err := qtx.GetGameForUpdate(ctx, int32(gameID))
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlcdb.Game{}, apierr.New(apierr.ErrGameNotFound, "game not found")
	}
	if err != nil {
		return sqlcdb.Game{}, err
	}

	if game.Status == gameStatusCancelled {
		return sqlcdb.Game{}, apierr.New(apierr.ErrGameAlreadyCancelled, "cannot leave a cancelled game")
	}
	if game.Status != gameStatusPending {
		return sqlcdb.Game{}, apierr.New(apierr.ErrGameAlreadyStarted, "cannot leave a non-pending game")
	}

	if game.CreatorID == userID {
		return sqlcdb.Game{}, apierr.New(apierr.ErrCreatorCannotLeave, "game creator cannot leave; cancel the game instead")
	}

	rows, err := qtx.RemoveGameParticipant(ctx, sqlcdb.RemoveGameParticipantParams{
		GameID: game.ID,
		UserID: userID,
	})
	if err != nil {
		return sqlcdb.Game{}, err
	}
	if rows == 0 {
		return sqlcdb.Game{}, apierr.New(apierr.ErrNotParticipant, "not a participant of this game")
	}

	if err := tx.Commit(ctx); err != nil {
		return sqlcdb.Game{}, err
	}

	return game, nil
}

func (s *GameService) DeleteGame(ctx context.Context, id int, userID uuid.UUID) error {
	game, err := s.q.GetGameByID(ctx, int32(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return apierr.New(apierr.ErrGameNotFound, "game not found")
	}
	if err != nil {
		return err
	}
	if game.CreatorID != userID {
		return apierr.New(apierr.ErrNotGameCreator, "only the game creator can delete the game")
	}
	rowsAff, err := s.q.DeleteGame(ctx, int32(id))
	if err != nil {
		return err
	}
	if rowsAff == 0 {
		return apierr.New(apierr.ErrGameNotFound, "game not found")
	}
	return nil
}
