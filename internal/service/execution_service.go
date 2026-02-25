package service

import (
	"context"

	"bytebattle/internal/executor"
)

type ExecutionService struct {
	exec executor.Executor
}

func NewExecutionService(exec executor.Executor) *ExecutionService {
	return &ExecutionService{exec: exec}
}

func (s *ExecutionService) Execute(ctx context.Context, req executor.ExecutionRequest) (executor.ExecutionResult, error) {
	return s.exec.Run(ctx, req)
}
