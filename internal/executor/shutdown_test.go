package executor

import (
	"context"
	"io"
	"sync"
	"testing"

	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
)

// fullMockDockerClient extends mockDockerClient with a configurable ContainerRemove.
type fullMockDockerClient struct {
	imageInspectFn    func(ctx context.Context, imageID string, opts ...client.ImageInspectOption) (image.InspectResponse, error)
	imagePullFn       func(ctx context.Context, refStr string, options image.PullOptions) (io.ReadCloser, error)
	containerRemoveFn func(ctx context.Context, id string, opts container.RemoveOptions) error
}

func (m *fullMockDockerClient) ImageInspect(ctx context.Context, id string, opts ...client.ImageInspectOption) (image.InspectResponse, error) {
	if m.imageInspectFn != nil {
		return m.imageInspectFn(ctx, id, opts...)
	}
	return image.InspectResponse{}, nil
}
func (m *fullMockDockerClient) ImagePull(ctx context.Context, ref string, opts image.PullOptions) (io.ReadCloser, error) {
	if m.imagePullFn != nil {
		return m.imagePullFn(ctx, ref, opts)
	}
	return io.NopCloser(nil), nil
}
func (m *fullMockDockerClient) ContainerCreate(_ context.Context, _ *container.Config, _ *container.HostConfig, _ *network.NetworkingConfig, _ *ocispec.Platform, _ string) (container.CreateResponse, error) {
	return container.CreateResponse{}, nil
}
func (m *fullMockDockerClient) ContainerStart(_ context.Context, _ string, _ container.StartOptions) error {
	return nil
}
func (m *fullMockDockerClient) ContainerRemove(ctx context.Context, id string, opts container.RemoveOptions) error {
	if m.containerRemoveFn != nil {
		return m.containerRemoveFn(ctx, id, opts)
	}
	return nil
}
func (m *fullMockDockerClient) ContainerExecCreate(_ context.Context, _ string, _ container.ExecOptions) (container.ExecCreateResponse, error) {
	return container.ExecCreateResponse{}, nil
}
func (m *fullMockDockerClient) ContainerExecAttach(_ context.Context, _ string, _ container.ExecStartOptions) (dockertypes.HijackedResponse, error) {
	return dockertypes.HijackedResponse{}, nil
}
func (m *fullMockDockerClient) ContainerExecInspect(_ context.Context, _ string) (container.ExecInspect, error) {
	return container.ExecInspect{}, nil
}
func (m *fullMockDockerClient) CopyToContainer(_ context.Context, _, _ string, _ io.Reader, _ container.CopyToContainerOptions) error {
	return nil
}

func newShutdownExecutor(cli dockerClient) *DockerExecutor {
	ctx, cancel := context.WithCancel(context.Background())
	return &DockerExecutor{
		cli:            cli,
		config:         &Config{Languages: map[Language]LangSettings{"python": {Image: "python:3.14-slim"}}},
		pools:          map[Language]chan string{"python": make(chan string, poolSize)},
		errChan:        make(chan error, 16),
		shutdownCtx:    ctx,
		shutdownCancel: cancel,
	}
}

// TestShutdown_DrainsPoolAndRemovesContainers verifies that warm containers sitting
// in the pool channel are removed when Shutdown is called.
func TestShutdown_DrainsPoolAndRemovesContainers(t *testing.T) {
	var mu sync.Mutex
	var removed []string

	mock := &fullMockDockerClient{
		containerRemoveFn: func(_ context.Context, id string, _ container.RemoveOptions) error {
			mu.Lock()
			removed = append(removed, id)
			mu.Unlock()
			return nil
		},
	}

	e := newShutdownExecutor(mock)
	e.pools["python"] <- "c1"
	e.pools["python"] <- "c2"
	e.pools["python"] <- "c3"

	e.Shutdown(context.Background())

	assert.ElementsMatch(t, []string{"c1", "c2", "c3"}, removed)
}

// TestShutdown_NoPanicOnTimeout verifies that passing an already-cancelled context
// does not panic (e.g. no send-on-closed-channel).
func TestShutdown_NoPanicOnTimeout(t *testing.T) {
	e := newShutdownExecutor(&fullMockDockerClient{})
	e.pools["python"] <- "c1"

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // simulate immediate timeout

	assert.NotPanics(t, func() {
		e.Shutdown(ctx)
	})
}
