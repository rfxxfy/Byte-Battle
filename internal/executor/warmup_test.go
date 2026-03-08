package executor

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// hijackedFromString returns a HijackedResponse whose reader yields data then EOF.
func hijackedFromString(data string) dockertypes.HijackedResponse {
	server, client := net.Pipe()
	go func() {
		_, _ = server.Write([]byte(data))
		_ = server.Close()
	}()
	return dockertypes.HijackedResponse{
		Conn:   client,
		Reader: bufio.NewReader(client),
	}
}

// hijackedBlocking returns a HijackedResponse that blocks forever (never sends EOF).
func hijackedBlocking() (resp dockertypes.HijackedResponse, cleanup func()) {
	server, client := net.Pipe()
	return dockertypes.HijackedResponse{
		Conn:   client,
		Reader: bufio.NewReader(client),
	}, func() { _ = server.Close(); _ = client.Close() }
}

func TestRunWarmup_Success(t *testing.T) {
	mock := &mockDockerClient{
		containerExecAttachFn: func(_ context.Context, _ string, _ container.ExecStartOptions) (dockertypes.HijackedResponse, error) {
			return hijackedFromString(""), nil
		},
		containerExecInspectFn: func(_ context.Context, _ string) (container.ExecInspect, error) {
			return container.ExecInspect{ExitCode: 0}, nil
		},
	}

	e := newTestExecutor(mock)
	err := e.runWarmup(context.Background(), "cid", "echo hi")
	require.NoError(t, err)
}

func TestRunWarmup_NonZeroExitCode(t *testing.T) {
	mock := &mockDockerClient{
		containerExecAttachFn: func(_ context.Context, _ string, _ container.ExecStartOptions) (dockertypes.HijackedResponse, error) {
			return hijackedFromString(""), nil
		},
		containerExecInspectFn: func(_ context.Context, _ string) (container.ExecInspect, error) {
			return container.ExecInspect{ExitCode: 1}, nil
		},
	}

	e := newTestExecutor(mock)
	err := e.runWarmup(context.Background(), "cid", "exit 1")
	assert.ErrorContains(t, err, "warmup exited with code 1")
}

func TestRunWarmup_ExecCreateError(t *testing.T) {
	mock := &mockDockerClient{
		containerExecCreateFn: func(_ context.Context, _ string, _ container.ExecOptions) (container.ExecCreateResponse, error) {
			return container.ExecCreateResponse{}, errors.New("daemon error")
		},
	}

	e := newTestExecutor(mock)
	err := e.runWarmup(context.Background(), "cid", "echo hi")
	assert.ErrorContains(t, err, "daemon error")
}

func TestRunWarmup_AttachError(t *testing.T) {
	mock := &mockDockerClient{
		containerExecAttachFn: func(_ context.Context, _ string, _ container.ExecStartOptions) (dockertypes.HijackedResponse, error) {
			return dockertypes.HijackedResponse{}, errors.New("attach failed")
		},
	}

	e := newTestExecutor(mock)
	err := e.runWarmup(context.Background(), "cid", "echo hi")
	assert.ErrorContains(t, err, "attach failed")
}

func TestRunWarmup_Timeout(t *testing.T) {
	resp, cleanup := hijackedBlocking()
	defer cleanup()

	mock := &mockDockerClient{
		containerExecAttachFn: func(_ context.Context, _ string, _ container.ExecStartOptions) (dockertypes.HijackedResponse, error) {
			return resp, nil
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	e := newTestExecutor(mock)
	start := time.Now()
	err := e.runWarmup(ctx, "cid", "sleep 9999")
	assert.WithinDuration(t, start, time.Now(), time.Second, "should return quickly on timeout")
	assert.Error(t, err)
}

func TestCreateWarmContainer_CallsWarmup(t *testing.T) {
	warmupCalled := false
	mock := &mockDockerClient{
		containerExecCreateFn: func(_ context.Context, _ string, cfg container.ExecOptions) (container.ExecCreateResponse, error) {
			if len(cfg.Cmd) > 0 && cfg.Cmd[0] == "/bin/sh" {
				warmupCalled = true
			}
			return container.ExecCreateResponse{ID: "exec-id"}, nil
		},
		containerExecAttachFn: func(_ context.Context, _ string, _ container.ExecStartOptions) (dockertypes.HijackedResponse, error) {
			return hijackedFromString(""), nil
		},
	}

	e := newTestExecutor(mock)
	settings := &LangSettings{
		Image:     "golang:1.26-alpine",
		WarmupCmd: `echo "warmup"`,
	}

	id, err := e.createWarmContainer(context.Background(), settings)
	require.NoError(t, err)
	assert.Equal(t, "test-container-id", id)
	assert.True(t, warmupCalled, "warmup exec should have been called")
}

func TestCreateWarmContainer_SkipsWarmupWhenEmpty(t *testing.T) {
	execCalled := false
	mock := &mockDockerClient{
		containerExecCreateFn: func(_ context.Context, _ string, _ container.ExecOptions) (container.ExecCreateResponse, error) {
			execCalled = true
			return container.ExecCreateResponse{}, nil
		},
	}

	e := newTestExecutor(mock)
	settings := &LangSettings{
		Image:     "python:3.14-slim",
		WarmupCmd: "",
	}

	_, err := e.createWarmContainer(context.Background(), settings)
	require.NoError(t, err)
	assert.False(t, execCalled, "exec should not be called when WarmupCmd is empty")
}

func TestRunWarmup_ReadsStdout(t *testing.T) {
	mock := &mockDockerClient{
		containerExecAttachFn: func(_ context.Context, _ string, _ container.ExecStartOptions) (dockertypes.HijackedResponse, error) {
			return hijackedFromString(strings.Repeat("x", 1024)), nil
		},
		containerExecInspectFn: func(_ context.Context, _ string) (container.ExecInspect, error) {
			return container.ExecInspect{ExitCode: 0}, nil
		},
	}

	e := newTestExecutor(mock)
	// should not block or fail even with output
	err := e.runWarmup(context.Background(), "cid", "cat /dev/urandom")
	require.NoError(t, err)
}

// Verify that runWarmup's own 60s timeout doesn't interfere with a fast warmup.
func TestRunWarmup_InternalTimeoutDoesNotAffectFastRun(t *testing.T) {
	mock := &mockDockerClient{
		containerExecAttachFn: func(_ context.Context, _ string, _ container.ExecStartOptions) (dockertypes.HijackedResponse, error) {
			return hijackedFromString(""), nil
		},
		containerExecInspectFn: func(_ context.Context, _ string) (container.ExecInspect, error) {
			return container.ExecInspect{ExitCode: 0}, nil
		},
	}

	e := newTestExecutor(mock)
	// Should complete well under 1 second
	done := make(chan error, 1)
	go func() {
		done <- e.runWarmup(context.Background(), "cid", "echo hi")
	}()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("runWarmup took too long")
	}
}

// Ensure runWarmup discards output without returning an error for large output.
func TestRunWarmup_LargeOutput(t *testing.T) {
	large := strings.Repeat("a", 100*1024)
	mock := &mockDockerClient{
		containerExecAttachFn: func(_ context.Context, _ string, _ container.ExecStartOptions) (dockertypes.HijackedResponse, error) {
			server, client := net.Pipe()
			go func() {
				_, _ = io.WriteString(server, large)
				_ = server.Close()
			}()
			return dockertypes.HijackedResponse{
				Conn:   client,
				Reader: bufio.NewReader(client),
			}, nil
		},
	}

	e := newTestExecutor(mock)
	err := e.runWarmup(context.Background(), "cid", "cmd")
	require.NoError(t, err)
}
