package service

import (
	"context"
	"errors"
	"fmt"

	"bytebattle/internal/apierr"
	"bytebattle/internal/executor"
	"bytebattle/internal/problems"

	"github.com/google/uuid"
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
}

type executionOutcome struct {
	accepted   bool
	failedTest *int
	stdout     string
	stderr     string
}

func NewSubmissionService(execSvc *ExecutionService, gameSvc *GameService, loader *problems.Loader) *SubmissionService {
	return &SubmissionService{execSvc: execSvc, gameSvc: gameSvc, problems: loader}
}

func (s *SubmissionService) Submit(ctx context.Context, gameID int, userID uuid.UUID, code string, language executor.Language) (SubmissionResult, error) {
	if !s.execSvc.TryAcquireSlot(userID) {
		return SubmissionResult{}, apierr.New(apierr.ErrExecutionInProgress, "execution already in progress")
	}
	defer s.execSvc.ReleaseSlot(userID)

	if err := s.execSvc.CheckRateLimit(userID); err != nil {
		return SubmissionResult{}, err
	}

	problem, expectedProblemIndex, err := s.getCurrentProblemForSubmission(ctx, gameID)
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

	return s.completeAcceptedSubmission(ctx, gameID, userID, expectedProblemIndex)
}

func (s *SubmissionService) getCurrentProblemForSubmission(ctx context.Context, gameID int) (*problems.Problem, int32, error) {
	game, err := s.gameSvc.GetGame(ctx, gameID)
	if err != nil {
		return nil, 0, fmt.Errorf("get game: %w", err)
	}
	expectedProblemIndex := game.CurrentProblemIndex

	problemID, err := s.gameSvc.GetGameProblemIDByIndex(ctx, int32(gameID), expectedProblemIndex)
	if err != nil {
		return nil, 0, fmt.Errorf("get game problem by index: %w", err)
	}

	problem, err := s.problems.Get(problemID)
	if err != nil {
		return nil, 0, fmt.Errorf("get problem: %w", err)
	}
	return problem, expectedProblemIndex, nil
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
	expectedProblemIndex int32,
) (SubmissionResult, error) {
	updatedGame, finished, err := s.gameSvc.HandleAcceptedSubmission(ctx, gameID, userID, expectedProblemIndex)
	if err != nil {
		var appErr *apierr.AppError
		if errors.As(err, &appErr) && (appErr.ErrorCode == apierr.ErrGameNotInProgress || appErr.ErrorCode == apierr.ErrRoundAlreadyAdvanced) {
			// Another player already won or advanced the round — skip broadcast,
			// the winner's goroutine already sent round_advanced / game_finished.
			return SubmissionResult{Accepted: true, AlreadyAdvanced: true}, nil
		}
		return SubmissionResult{}, fmt.Errorf("complete game: %w", err)
	}

	if !finished {
		nextProblemID, nextProblemErr := s.gameSvc.GetGameProblemIDByIndex(ctx, int32(gameID), updatedGame.CurrentProblemIndex)
		if nextProblemErr != nil {
			return SubmissionResult{}, fmt.Errorf("get next game problem by index: %w", nextProblemErr)
		}
		return SubmissionResult{
			Accepted:   true,
			ProblemID:  nextProblemID,
			ProblemIdx: int(updatedGame.CurrentProblemIndex),
		}, nil
	}

	return SubmissionResult{Accepted: true, WinnerID: updatedGame.WinnerID.UUID}, nil
}
