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
	Accepted   bool
	FailedTest *int
	Stdout     string
	Stderr     string
	WinnerID   uuid.UUID
}

type SubmissionService struct {
	execSvc  *ExecutionService
	gameSvc  *GameService
	problems *problems.Loader
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

	game, err := s.gameSvc.GetGame(ctx, gameID)
	if err != nil {
		return SubmissionResult{}, fmt.Errorf("get game: %w", err)
	}

	problem, err := s.problems.Get(game.ProblemID)
	if err != nil {
		return SubmissionResult{}, fmt.Errorf("get problem: %w", err)
	}

	accepted := true
	var failedTest *int
	var stdout, stderr string

	for i, tc := range problem.TestCases {
		result, execErr := s.execSvc.Execute(ctx, executor.ExecutionRequest{
			Code:     code,
			Language: language,
			Stdin:    tc.Input,
		})
		stdout = result.Stdout
		stderr = result.Stderr

		if execErr != nil || !problems.Match(result.Stdout, tc.Expected) {
			accepted = false
			idx := i
			failedTest = &idx
			break
		}
	}

	if !accepted {
		return SubmissionResult{
			Accepted:   false,
			FailedTest: failedTest,
			Stdout:     stdout,
			Stderr:     stderr,
		}, nil
	}

	completed, err := s.gameSvc.CompleteGameAsWinner(ctx, gameID, userID)
	if err != nil {
		var appErr *apierr.AppError
		if errors.As(err, &appErr) && appErr.ErrorCode == apierr.ErrGameNotInProgress {
			// Another player already won — submission is still accepted but we
			// don't set WinnerID (the other player's submit already broadcast it).
			return SubmissionResult{Accepted: true}, nil
		}
		return SubmissionResult{}, fmt.Errorf("complete game: %w", err)
	}

	return SubmissionResult{Accepted: true, WinnerID: completed.WinnerID.UUID}, nil
}
