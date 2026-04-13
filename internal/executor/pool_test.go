package executor

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsContainerRunning_InspectError(t *testing.T) {
	mock := &mockDockerClient{
		containerInspectFn: func(_ context.Context, _ string) (container.InspectResponse, error) {
			return container.InspectResponse{}, assert.AnError
		},
	}
	e := newTestExecutor(mock)
	require.False(t, e.isContainerRunning(context.Background(), "any"))
}

func TestIsContainerRunning_NilState(t *testing.T) {
	mock := &mockDockerClient{
		containerInspectFn: func(_ context.Context, _ string) (container.InspectResponse, error) {
			return container.InspectResponse{}, nil
		},
	}
	e := newTestExecutor(mock)
	require.False(t, e.isContainerRunning(context.Background(), "any"))
}

func TestInitPools_CustomPoolSize(t *testing.T) {
	shutdownCtx, shutdown := context.WithCancel(context.Background())
	shutdown() // cancel immediately so spawned workers exit without creating containers
	e := &DockerExecutor{
		cli: &mockDockerClient{},
		config: &Config{Languages: map[Language]LangSettings{
			"python": {Image: "python:3.14-slim", PoolSize: 7},
			"go":     {Image: "golang:1.26-alpine"},
		}},
		pools:       make(map[Language]*langPool),
		errChan:     make(chan error, 16),
		shutdownCtx: shutdownCtx,
		shutdown:    shutdown,
	}
	e.primedPerLang = make(map[Language]*atomic.Bool, len(e.config.Languages))
	for lang := range e.config.Languages {
		e.primedPerLang[lang] = new(atomic.Bool)
	}

	e.initPools()

	assert.Equal(t, 7*queueMultiplier, cap(e.pools["python"].queue), "custom PoolSize should set queue capacity")
	assert.Equal(t, poolSize*queueMultiplier, cap(e.pools["go"].queue), "zero PoolSize should fall back to global poolSize")
}

func TestIsReady_FalseBeforePrimed(t *testing.T) {
	e := newTestExecutor(&mockDockerClient{})
	e.pools["python"] = &langPool{}
	assert.False(t, e.IsReady())
}

func TestIsReady_TrueAfterAllPoolsPrimed(t *testing.T) {
	e := newTestExecutor(&mockDockerClient{})
	e.pools["python"] = &langPool{}
	e.notifyPoolPrimed("python")
	assert.True(t, e.IsReady())
}

func TestIsReady_FalseWithEmptyPools(t *testing.T) {
	e := newTestExecutor(&mockDockerClient{})
	e.pools = make(map[Language]*langPool)
	e.primedPerLang = make(map[Language]*atomic.Bool)
	e.config = &Config{Languages: map[Language]LangSettings{}}
	assert.False(t, e.IsReady())
}

func TestIsReady_StaysReadyAfterPrimed(t *testing.T) {
	e := newTestExecutor(&mockDockerClient{})
	e.pools["python"] = &langPool{}
	e.notifyPoolPrimed("python")
	require.True(t, e.IsReady())
	assert.True(t, e.IsReady())
}

func TestIsReady_MultiLang_NotReadyUntilAllPrimed(t *testing.T) {
	langs := map[Language]LangSettings{
		"python": {Image: "python:3.14-slim"},
		"go":     {Image: "golang:1.26-alpine"},
	}
	e := newTestExecutorWithLangs(&mockDockerClient{}, langs)
	e.pools["python"] = &langPool{}
	e.pools["go"] = &langPool{}

	e.notifyPoolPrimed("python")
	assert.False(t, e.IsReady(), "should not be ready until all languages primed")

	e.notifyPoolPrimed("go")
	assert.True(t, e.IsReady())
}
