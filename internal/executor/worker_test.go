package executor

import (
	"context"
	"errors"
	"testing"
	"time"

	"bytebattle/internal/apierr"

	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRun_UnsupportedLanguage(t *testing.T) {
	e := newTestExecutor(&mockDockerClient{})
	_, err := e.Run(context.Background(), ExecutionRequest{Language: "cobol"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported language")
}

func TestRun_BackpressureWhenQueueFull(t *testing.T) {
	e := newTestExecutor(&mockDockerClient{})
	queue := make(chan workItem, 1)
	e.pools["python"] = &langPool{queue: queue, settings: LangSettings{Image: "python:3.14-slim"}}
	e.notifyPoolPrimed("python")

	queue <- workItem{ctx: context.Background(), req: ExecutionRequest{Language: "python"}, result: make(chan workResult, 1)}

	_, err := e.Run(context.Background(), ExecutionRequest{Language: "python"})
	require.Error(t, err)

	var appErr *apierr.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apierr.ErrExecutorOverloaded, appErr.ErrorCode)
}

func TestRun_ShuttingDownRejectsImmediately(t *testing.T) {
	e := newTestExecutor(&mockDockerClient{})
	e.pools["python"] = &langPool{queue: make(chan workItem, 4)}
	e.notifyPoolPrimed("python")

	e.shutdown()

	_, err := e.Run(context.Background(), ExecutionRequest{Language: "python"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "shutting down")
}

func TestRun_CallerCancelCancels(t *testing.T) {
	e := newTestExecutor(&mockDockerClient{})
	e.pools["python"] = &langPool{queue: make(chan workItem, 4)}
	e.notifyPoolPrimed("python")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := e.Run(ctx, ExecutionRequest{Language: "python"})
	require.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled))
}

func TestDrainQueueWithShutdown(t *testing.T) {
	e := newTestExecutor(&mockDockerClient{})

	result1 := make(chan workResult, 1)
	result2 := make(chan workResult, 1)
	lp := &langPool{queue: make(chan workItem, 4)}
	lp.queue <- workItem{ctx: context.Background(), req: ExecutionRequest{Language: "python"}, result: result1}
	lp.queue <- workItem{ctx: context.Background(), req: ExecutionRequest{Language: "python"}, result: result2}

	e.drainQueueWithShutdown(lp)

	r1 := <-result1
	r2 := <-result2
	require.Error(t, r1.err)
	require.Error(t, r2.err)
	assert.Contains(t, r1.err.Error(), "shutting down")
	assert.Contains(t, r2.err.Error(), "shutting down")
	assert.Empty(t, lp.queue)
}

func TestBuildShellCommand_WrapsRunWithTimeout(t *testing.T) {
	e := newTestExecutor(&mockDockerClient{})
	cfg := &LangSettings{RunCmd: []string{"python", "main.py"}}
	assert.Equal(t, "timeout 10s python main.py", e.buildShellCommand(cfg, false, 10*time.Second))
}

func TestBuildShellCommand_CompileAndRun(t *testing.T) {
	e := newTestExecutor(&mockDockerClient{})
	cfg := &LangSettings{
		CompileCmd: []string{"g++", "-O2", "main.cpp", "-o", "main"},
		RunCmd:     []string{"./main"},
	}
	assert.Equal(t, "g++ -O2 main.cpp -o main && timeout 5s ./main", e.buildShellCommand(cfg, false, 5*time.Second))
}

func TestBuildShellCommand_StdinRedirect(t *testing.T) {
	e := newTestExecutor(&mockDockerClient{})
	cfg := &LangSettings{RunCmd: []string{"python", "main.py"}}
	assert.Equal(t, "timeout 10s python main.py < input.txt", e.buildShellCommand(cfg, true, 10*time.Second))
}

func TestCleanWorkDir_CallsCorrectCommand(t *testing.T) {
	var capturedCmd []string
	mock := &mockDockerClient{
		containerExecCreateFn: func(_ context.Context, _ string, cfg container.ExecOptions) (container.ExecCreateResponse, error) {
			capturedCmd = cfg.Cmd
			return container.ExecCreateResponse{ID: "exec-id"}, nil
		},
		containerExecAttachFn: func(_ context.Context, _ string, _ container.ExecStartOptions) (dockertypes.HijackedResponse, error) {
			return hijackedFromString(""), nil
		},
	}
	e := newTestExecutor(mock)
	err := e.cleanWorkDir(context.Background(), "container-id")
	require.NoError(t, err)
	assert.Equal(t, []string{"/bin/sh", "-c", "rm -rf /app/* /tmp/*"}, capturedCmd)
}
