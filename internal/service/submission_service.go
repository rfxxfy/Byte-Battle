package service

import (
	"context"
	"errors"
	"fmt"
	"log"

	"bytebattle/internal/apierr"
	sqlcdb "bytebattle/internal/db/sqlc"
	"bytebattle/internal/executor"
	"bytebattle/internal/problems"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type SubmissionResult struct {
	Accepted        bool
	AlreadyAdvanced bool
	FailedTest      *int
	Stdout          string
	Stderr          string
	WinnerID        uuid.UUID
	ProblemID       string
	ProblemIdx      int
}

type SubmissionService struct {
	execSvc  *ExecutionService
	gameSvc  *GameService
	problems *problems.Loader
	q        sqlcdb.Querier
}

type executionOutcome struct {
	accepted   bool
	failedTest *int
	stdout     string
	stderr     string
}

func NewSubmissionService(execSvc *ExecutionService, gameSvc *GameService, loader *problems.Loader, q sqlcdb.Querier) *SubmissionService {
	return &SubmissionService{execSvc: execSvc, gameSvc: gameSvc, problems: loader, q: q}
}

func (s *SubmissionService) Submit(ctx context.Context, gameID int, userID uuid.UUID, code string, language executor.Language) (SubmissionResult, error) {
	if !s.execSvc.TryAcquireSlot(userID) {
		return SubmissionResult{}, apierr.New(apierr.ErrExecutionInProgress, "execution already in progress")
	}
	defer s.execSvc.ReleaseSlot(userID)

	if err := s.execSvc.CheckRateLimit(userID); err != nil {
		return SubmissionResult{}, err
	}

	problem, err := s.getCurrentProblemForSubmission(ctx, gameID, userID)
	if err != nil {
		return SubmissionResult{}, err
	}

	outcome := s.executeAgainstProblem(ctx, problem, code, language)
	if !outcome.accepted {
		return SubmissionResult{
			Accepted:   false,
			FailedTest: outcome.failedTest,
			Stdout:     outcome.stdout,
			Stderr:     outcome.stderr,
		}, nil
	}

	if err := s.q.InsertSolution(ctx, sqlcdb.InsertSolutionParams{
		UserID:    userID,
		ProblemID: problem.ID,
		GameID:    pgtype.Int4{Int32: int32(gameID), Valid: true},
		Code:      code,
		Language:  string(language),
	}); err != nil {
		log.Printf("warn: failed to save solution user=%s problem=%s game=%d: %v", userID, problem.ID, gameID, err)
	}

	return s.completeAcceptedSubmission(ctx, gameID, userID)
}

func (s *SubmissionService) getCurrentProblemForSubmission(ctx context.Context, gameID int, userID uuid.UUID) (*problems.Problem, error) {
	game, err := s.gameSvc.GetGame(ctx, gameID)
	if err != nil {
		return nil, fmt.Errorf("get game: %w", err)
	}
	if game.Status != "active" {
		return nil, apierr.New(apierr.ErrGameNotInProgress, "game is not in progress")
	}

	// Each player has their own current problem index.
	playerIdx, err := s.gameSvc.GetParticipantProblemIndex(ctx, gameID, userID)
	if err != nil {
		return nil, fmt.Errorf("get participant problem index: %w", err)
	}

	problemID, err := s.gameSvc.GetGameProblemIDByIndex(ctx, int32(gameID), playerIdx)
	if err != nil {
		return nil, fmt.Errorf("get game problem by index: %w", err)
	}

	problem, err := s.problems.Get(problemID)
	if err != nil {
		return nil, fmt.Errorf("get problem: %w", err)
	}
	return problem, nil
}

func (s *SubmissionService) executeAgainstProblem(
	ctx context.Context,
	problem *problems.Problem,
	code string,
	language executor.Language,
) executionOutcome {
	outcome := executionOutcome{accepted: true}

	for i, tc := range problem.TestCases {
		result, execErr := s.execSvc.Execute(ctx, executor.ExecutionRequest{
			Code:     code,
			Language: language,
			Stdin:    tc.Input,
		})
		outcome.stdout = result.Stdout
		outcome.stderr = result.Stderr

		if execErr != nil || !problems.Match(result.Stdout, tc.Expected) {
			outcome.accepted = false
			idx := i
			outcome.failedTest = &idx
			break
		}
	}

	return outcome
}

func (s *SubmissionService) completeAcceptedSubmission(
	ctx context.Context,
	gameID int,
	userID uuid.UUID,
) (SubmissionResult, error) {
	updatedGame, finished, err := s.gameSvc.HandleAcceptedSubmission(ctx, gameID, userID)
	if err != nil {
		if errors.Is(err, errGameAlreadyFinished) {
			return SubmissionResult{Accepted: true, AlreadyAdvanced: true}, nil
		}
		return SubmissionResult{}, fmt.Errorf("complete game: %w", err)
	}

	if finished {
		return SubmissionResult{Accepted: true, WinnerID: updatedGame.WinnerID.UUID}, nil
	}

	// Player advanced to their next problem.
	playerIdx, err := s.gameSvc.GetParticipantProblemIndex(ctx, gameID, userID)
	if err != nil {
		return SubmissionResult{}, fmt.Errorf("get participant problem index: %w", err)
	}
	nextProblemID, err := s.gameSvc.GetGameProblemIDByIndex(ctx, int32(gameID), playerIdx)
	if err != nil {
		return SubmissionResult{}, fmt.Errorf("get next problem: %w", err)
	}
	return SubmissionResult{
		Accepted:   true,
		ProblemID:  nextProblemID,
		ProblemIdx: int(playerIdx),
	}, nil
}
