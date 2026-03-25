package executor

import (
	"context"
	"sync"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/assert"
)

func TestCleanupPreviousPools_RemovesLabelledContainers(t *testing.T) {
	var mu sync.Mutex
	var removed []string

	mock := &mockDockerClient{
		containerListFn: func(_ context.Context, opts container.ListOptions) ([]container.Summary, error) {
			// verify the label filter is set correctly
			vals := opts.Filters.Get("label")
			assert.Contains(t, vals, poolLabelKey+"="+poolLabelVal)
			return []container.Summary{
				{ID: "old-c1"},
				{ID: "old-c2"},
			}, nil
		},
		containerRemoveFn: func(_ context.Context, id string, _ container.RemoveOptions) error {
			mu.Lock()
			removed = append(removed, id)
			mu.Unlock()
			return nil
		},
	}

	e := newTestExecutor(mock)
	e.cleanupPreviousPools(context.Background())

	assert.ElementsMatch(t, []string{"old-c1", "old-c2"}, removed)
}

func TestCleanupPreviousPools_NoErrorOnListFailure(t *testing.T) {
	mock := &mockDockerClient{
		containerListFn: func(_ context.Context, _ container.ListOptions) ([]container.Summary, error) {
			return nil, assert.AnError
		},
	}

	e := newTestExecutor(mock)
	// should log and return without panic
	assert.NotPanics(t, func() {
		e.cleanupPreviousPools(context.Background())
	})
}
