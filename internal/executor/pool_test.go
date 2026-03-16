package executor

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTakeFromPool_ReturnsLiveContainer(t *testing.T) {
	mock := &mockDockerClient{}
	e := newTestExecutor(mock)
	e.pools["python"] = make(chan string, 3)
	e.pools["python"] <- "alive"

	id := e.takeFromPool(context.Background(), "python")
	assert.Equal(t, "alive", id)
}

func TestTakeFromPool_DiscardsDeadReturnsNext(t *testing.T) {
	calls := 0
	mock := &mockDockerClient{
		containerInspectFn: func(_ context.Context, id string) (container.InspectResponse, error) {
			calls++
			if id == "dead" {
				return container.InspectResponse{
					ContainerJSONBase: &container.ContainerJSONBase{
						State: &container.State{Running: false},
					},
				}, nil
			}
			return container.InspectResponse{
				ContainerJSONBase: &container.ContainerJSONBase{
					State: &container.State{Running: true},
				},
			}, nil
		},
	}
	e := newTestExecutor(mock)
	e.pools["python"] = make(chan string, 3)
	e.pools["python"] <- "dead"
	e.pools["python"] <- "alive"

	id := e.takeFromPool(context.Background(), "python")
	assert.Equal(t, "alive", id)
	assert.Equal(t, 2, calls)
}

func TestTakeFromPool_EmptyPoolReturnsEmpty(t *testing.T) {
	e := newTestExecutor(&mockDockerClient{})
	e.pools["python"] = make(chan string, 3)

	id := e.takeFromPool(context.Background(), "python")
	assert.Empty(t, id)
}

func TestTakeFromPool_AllDeadReturnsEmpty(t *testing.T) {
	mock := &mockDockerClient{
		containerInspectFn: func(_ context.Context, _ string) (container.InspectResponse, error) {
			return container.InspectResponse{
				ContainerJSONBase: &container.ContainerJSONBase{
					State: &container.State{Running: false},
				},
			}, nil
		},
	}
	e := newTestExecutor(mock)
	e.pools["python"] = make(chan string, 3)
	e.pools["python"] <- "dead1"
	e.pools["python"] <- "dead2"

	id := e.takeFromPool(context.Background(), "python")
	assert.Empty(t, id)
}

func TestIsContainerRunning_InspectError(t *testing.T) {
	mock := &mockDockerClient{
		containerInspectFn: func(_ context.Context, _ string) (container.InspectResponse, error) {
			return container.InspectResponse{}, errors.New("daemon gone")
		},
	}
	e := newTestExecutor(mock)
	require.False(t, e.isContainerRunning(context.Background(), "any"))
}

func TestInitPools_CustomPoolSize(t *testing.T) {
	e := &DockerExecutor{
		cli: &mockDockerClient{},
		config: &Config{Languages: map[Language]LangSettings{
			"python": {Image: "python:3.14-slim", PoolSize: 7},
			"go":     {Image: "golang:1.26-alpine"},
		}},
		pools:   make(map[Language]chan string),
		errChan: make(chan error, 16),
	}
	e.initPools()
	assert.Equal(t, 7, cap(e.pools["python"]), "custom PoolSize should set channel capacity")
	assert.Equal(t, poolSize, cap(e.pools["go"]), "zero PoolSize should fall back to global poolSize")
}

func TestIsContainerRunning_NilState(t *testing.T) {
	mock := &mockDockerClient{
		containerInspectFn: func(_ context.Context, _ string) (container.InspectResponse, error) {
			return container.InspectResponse{}, nil // ContainerJSONBase is nil
		},
	}
	e := newTestExecutor(mock)
	require.False(t, e.isContainerRunning(context.Background(), "any"))
}

func TestIsReady_FalseBeforePrimed(t *testing.T) {
	e := newTestExecutor(&mockDockerClient{})
	e.pools["python"] = make(chan string, 3)
	assert.False(t, e.IsReady())
}

func TestIsReady_TrueAfterAllPoolsPrimed(t *testing.T) {
	e := newTestExecutor(&mockDockerClient{})
	e.pools["python"] = make(chan string, 3)
	e.notifyPoolPrimed("python")
	assert.True(t, e.IsReady())
}

func TestIsReady_FalseWithEmptyPools(t *testing.T) {
	e := newTestExecutor(&mockDockerClient{})
	// pools not initialized — no languages
	e.pools = make(map[Language]chan string)
	e.primedPerLang = make(map[Language]*atomic.Bool)
	e.config = &Config{Languages: map[Language]LangSettings{}}
	assert.False(t, e.IsReady())
}

func TestIsReady_NotFlapWhenPoolDrained(t *testing.T) {
	e := newTestExecutor(&mockDockerClient{})
	e.pools["python"] = make(chan string, 3)
	e.notifyPoolPrimed("python")
	require.True(t, e.IsReady())

	// drain the pool — should stay ready
	e.pools["python"] <- "c1"
	<-e.pools["python"]
	assert.True(t, e.IsReady(), "should not flap when pool is temporarily empty")
}

func TestIsReady_MultiLang_NotReadyUntilAllPrimed(t *testing.T) {
	langs := map[Language]LangSettings{
		"python": {Image: "python:3.14-slim"},
		"go":     {Image: "golang:1.26-alpine"},
	}
	e := newTestExecutorWithLangs(&mockDockerClient{}, langs)
	e.pools["python"] = make(chan string, 3)
	e.pools["go"] = make(chan string, 3)

	e.notifyPoolPrimed("python")
	assert.False(t, e.IsReady(), "should not be ready until all languages primed")

	e.notifyPoolPrimed("go")
	assert.True(t, e.IsReady())
}
