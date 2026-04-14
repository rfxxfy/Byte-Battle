package service

import (
	"context"
	"errors"
	"fmt"

	"bytebattle/internal/apierr"
	sqlcdb "bytebattle/internal/db/sqlc"
	"bytebattle/internal/problems"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
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
	maxGameProblems     = 20 // sync with CHECK (problem_index < 20) in migration 000009
)

type GameService struct {
	q        *sqlcdb.Queries
	pool     *pgxpool.Pool
	problems *problems.Loader
}

func NewGameService(q *sqlcdb.Queries, pool *pgxpool.Pool, loader *problems.Loader) *GameService {
	return &GameService{q: q, pool: pool, problems: loader}
}

func (s *GameService) CreateGame(ctx context.Context, creatorID uuid.UUID, problemIDs []string, isPublic bool) (sqlcdb.Game, error) {
	if len(problemIDs) == 0 {
		return sqlcdb.Game{}, apierr.New(apierr.ErrValidation, "at least one problem is required")
	}
	if len(problemIDs) > maxGameProblems {
		return sqlcdb.Game{}, apierr.New(apierr.ErrValidation, "too many problems in game")
	}
	for _, problemID := range problemIDs {
		if problemID == "" {
			return sqlcdb.Game{}, apierr.New(apierr.ErrValidation, "problem_id cannot be empty")
		}
		if _, err := s.problems.Get(problemID); err != nil {
			return sqlcdb.Game{}, apierr.New(apierr.ErrProblemNotFound, "problem not found")
		}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return sqlcdb.Game{}, err
	}
	defer tx.Rollback(ctx)

	qtx := s.q.WithTx(tx)

	game, err := qtx.CreateGame(ctx, sqlcdb.CreateGameParams{
		CreatorID: creatorID,
		IsPublic:  isPublic,
	})
	if err != nil {
		return sqlcdb.Game{}, err
	}

	for idx, problemID := range problemIDs {
		if err := qtx.AddGameProblem(ctx, sqlcdb.AddGameProblemParams{
			GameID:       game.ID,
			ProblemIndex: int32(idx),
			ProblemID:    problemID,
		}); err != nil {
			return sqlcdb.Game{}, err
		}
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

func (s *GameService) IsParticipant(ctx context.Context, gameID int, userID uuid.UUID) (bool, error) {
	return s.q.IsGameParticipant(ctx, sqlcdb.IsGameParticipantParams{
		GameID: int32(gameID),
		UserID: userID,
	})
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

func (s *GameService) GetGameProblemIDs(ctx context.Context, gameID int32) ([]string, error) {
	return s.q.GetGameProblemIDs(ctx, gameID)
}

func (s *GameService) GetGameProblemIDByIndex(ctx context.Context, gameID, problemIndex int32) (string, error) {
	id, err := s.q.GetGameProblemIDByIndex(ctx, sqlcdb.GetGameProblemIDByIndexParams{
		GameID:       gameID,
		ProblemIndex: problemIndex,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return "", apierr.New(apierr.ErrGameNotFound, "game problem not found")
	}
	return id, err
}

func (s *GameService) GetGameProblemIDsByGameIDs(ctx context.Context, gameIDs []int32) (map[int32][]string, error) {
	rows, err := s.q.GetGameProblemIDsByGameIDs(ctx, gameIDs)
	if err != nil {
		return nil, err
	}
	result := make(map[int32][]string, len(gameIDs))
	for _, row := range rows {
		result[row.GameID] = append(result[row.GameID], row.ProblemID)
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

func (s *GameService) CanAccessGame(ctx context.Context, game sqlcdb.Game, userID uuid.UUID) error {
	if game.IsPublic {
		return nil
	}
	ok, err := s.q.IsGameParticipant(ctx, sqlcdb.IsGameParticipantParams{
		GameID: game.ID,
		UserID: userID,
	})
	if err != nil {
		return err
	}
	if !ok {
		return apierr.New(apierr.ErrNotParticipant, "not a participant")
	}
	return nil
}

func (s *GameService) GetGameByToken(ctx context.Context, token uuid.UUID) (sqlcdb.Game, error) {
	game, err := s.q.GetGameByInviteToken(ctx, token)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlcdb.Game{}, apierr.New(apierr.ErrGameNotFound, "invalid invite token")
	}
	return game, err
}

func (s *GameService) JoinGameByToken(ctx context.Context, token, userID uuid.UUID) (sqlcdb.Game, error) {
	game, err := s.GetGameByToken(ctx, token)
	if err != nil {
		return sqlcdb.Game{}, err
	}
	return s.JoinGame(ctx, int(game.ID), userID)
}

func (s *GameService) ListGames(ctx context.Context, limit, offset int, userID uuid.UUID) ([]sqlcdb.Game, int64, error) {
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	total, err := s.q.CountGamesForUser(ctx, userID)
	if err != nil {
		return nil, 0, err
	}

	games, err := s.q.ListGamesForUser(ctx, sqlcdb.ListGamesForUserParams{
		Limit:   int32(limit),
		Offset:  int32(offset),
		Column3: userID,
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

func (s *GameService) CompleteGameAsWinner(ctx context.Context, id int, winnerID uuid.UUID) (sqlcdb.Game, error) {
	return s.completeGame(ctx, id, winnerID)
}

// HandleAcceptedSubmission advances the player's individual problem index.
// If the player has now solved all problems they win the game.
// Returns (game, finished, error). finished=true means this player won.
func (s *GameService) HandleAcceptedSubmission(ctx context.Context, id int, userID uuid.UUID) (sqlcdb.Game, bool, error) {
	// 1. Advance this player's problem index atomically.
	newIdx, err := s.q.AdvanceParticipantProblem(ctx, sqlcdb.AdvanceParticipantProblemParams{
		GameID: int32(id),
		UserID: userID,
	})
	if err != nil {
		return sqlcdb.Game{}, false, fmt.Errorf("advance participant problem: %w", err)
	}

	// 2. Check total problems.
	totalProblems, err := s.q.CountGameProblems(ctx, int32(id))
	if err != nil {
		return sqlcdb.Game{}, false, err
	}

	// 3. Not finished yet — return early.
	if totalProblems == 0 || int64(newIdx) < totalProblems {
		game, err := s.q.GetGameByID(ctx, int32(id))
		return game, false, err
	}

	// 4. Player solved all problems — try to claim the win inside a transaction.
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return sqlcdb.Game{}, false, err
	}
	defer tx.Rollback(ctx)

	qtx := s.q.WithTx(tx)
	game, err := qtx.GetGameForUpdate(ctx, int32(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlcdb.Game{}, false, apierr.New(apierr.ErrGameNotFound, "game not found")
	}
	if err != nil {
		return sqlcdb.Game{}, false, err
	}

	// Another player may have already finished.
	if game.Status != gameStatusActive {
		return game, false, apierr.New(apierr.ErrRoundAlreadyAdvanced, "game already finished")
	}

	updated, err := qtx.CompleteGame(ctx, sqlcdb.CompleteGameParams{
		ID:       game.ID,
		WinnerID: uuid.NullUUID{UUID: userID, Valid: true},
	})
	if err != nil {
		return sqlcdb.Game{}, false, err
	}

	if err := tx.Commit(ctx); err != nil {
		return sqlcdb.Game{}, false, err
	}

	return updated, true, nil
}

func (s *GameService) GetParticipantProblemIndex(ctx context.Context, gameID int, userID uuid.UUID) (int32, error) {
	idx, err := s.q.GetParticipantProblemIndex(ctx, sqlcdb.GetParticipantProblemIndexParams{
		GameID: int32(gameID),
		UserID: userID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, apierr.New(apierr.ErrNotParticipant, "not a participant")
	}
	return idx, err
}

func (s *GameService) GetAllParticipantsProblemIndices(ctx context.Context, gameID int) (map[string]int32, error) {
	rows, err := s.q.GetAllParticipantsProblemIndices(ctx, int32(gameID))
	if err != nil {
		return nil, err
	}
	result := make(map[string]int32, len(rows))
	for _, r := range rows {
		result[r.UserID.String()] = r.CurrentProblemIndex
	}
	return result, nil
}

func (s *GameService) CompleteGame(ctx context.Context, id int, callerID, winnerID uuid.UUID) (sqlcdb.Game, error) {
	game, err := s.q.GetGameByID(ctx, int32(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlcdb.Game{}, apierr.New(apierr.ErrGameNotFound, "game not found")
	}
	if err != nil {
		return sqlcdb.Game{}, err
	}
	if game.CreatorID != callerID {
		return sqlcdb.Game{}, apierr.New(apierr.ErrNotGameCreator, "only the game creator can complete the game")
	}
	return s.completeGame(ctx, id, winnerID)
}

func (s *GameService) completeGame(ctx context.Context, id int, winnerID uuid.UUID) (sqlcdb.Game, error) {
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

func (s *GameService) SetWinner(ctx context.Context, gameID int, winnerID uuid.UUID) error {
	return s.q.UpdateGameWinner(ctx, sqlcdb.UpdateGameWinnerParams{
		ID:       int32(gameID),
		WinnerID: uuid.NullUUID{UUID: winnerID, Valid: true},
	})
}

func (s *GameService) GetGameSolutions(ctx context.Context, gameID int) ([]sqlcdb.GetGameSolutionsRow, error) {
	game, err := s.q.GetGameByID(ctx, int32(gameID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apierr.New(apierr.ErrGameNotFound, "game not found")
	}
	if err != nil {
		return nil, err
	}
	if game.Status != gameStatusFinished {
		return nil, apierr.New(apierr.ErrGameNotFinished, "solutions are only available after the game is finished")
	}
	return s.q.GetGameSolutions(ctx, pgtype.Int4{Int32: int32(gameID), Valid: true})
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
