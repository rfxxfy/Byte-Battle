package service

import (
	"context"

	"bytebattle/internal/executor"
)

type ExecutionService struct {
	executor executor.Executor
}

func NewExecutionService(exec executor.Executor) *ExecutionService {
	return &ExecutionService{executor: exec}
}

func (s *ExecutionService) Execute(ctx context.Context, req executor.ExecutionRequest) (executor.ExecutionResult, error) {
	return s.executor.Run(ctx, req)
}
