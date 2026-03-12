package service

import (
	"context"
	"fmt"

	"bytebattle/internal/executor"
	"bytebattle/internal/problems"
)

type ExecutionService struct {
	executor executor.Executor
	problems *problems.Loader
}

func NewExecutionService(exec executor.Executor, loader *problems.Loader) *ExecutionService {
	return &ExecutionService{executor: exec, problems: loader}
}

func (s *ExecutionService) Execute(ctx context.Context, req executor.ExecutionRequest) (executor.ExecutionResult, error) {
	return s.executor.Run(ctx, req)
}

func (s *ExecutionService) ListProblems() ([]*problems.Problem, error) {
	if s.problems == nil {
		return nil, fmt.Errorf("problems loader is not configured")
	}
	return s.problems.List(), nil
}

func (s *ExecutionService) GetProblem(id string) (*problems.Problem, error) {
	if s.problems == nil {
		return nil, fmt.Errorf("problems loader is not configured")
	}
	return s.problems.Get(id)
}
