package executor

import (
	"context"
	"time"
)

type Language string

type ExecutionRequest struct {
	Code        string
	Language    Language
	Stdin       string
	TimeLimit   time.Duration
	MemoryLimit int64
}

type ExecutionResult struct {
	Stdout     string
	Stderr     string
	ExitCode   int
	TimeUsed   time.Duration
	MemoryUsed int64
	Error      error
}

type Executor interface {
	Run(ctx context.Context, req ExecutionRequest) (ExecutionResult, error)
}
