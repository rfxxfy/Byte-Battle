package executor

import (
	"context"
	"errors"
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

func TestIsContainerRunning_NilState(t *testing.T) {
	mock := &mockDockerClient{
		containerInspectFn: func(_ context.Context, _ string) (container.InspectResponse, error) {
			return container.InspectResponse{}, nil // ContainerJSONBase is nil
		},
	}
	e := newTestExecutor(mock)
	require.False(t, e.isContainerRunning(context.Background(), "any"))
}
