package executor

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dockertypes "github.com/docker/docker/api/types"
)

type mockDockerClient struct {
	imageInspectFn    func(ctx context.Context, imageID string, opts ...client.ImageInspectOption) (image.InspectResponse, error)
	imagePullFn       func(ctx context.Context, refStr string, options image.PullOptions) (io.ReadCloser, error)
	containerListFn   func(ctx context.Context, opts container.ListOptions) ([]container.Summary, error)
	containerRemoveFn func(ctx context.Context, id string, opts container.RemoveOptions) error
}

func (m *mockDockerClient) ImageInspect(ctx context.Context, imageID string, opts ...client.ImageInspectOption) (image.InspectResponse, error) {
	return m.imageInspectFn(ctx, imageID, opts...)
}

func (m *mockDockerClient) ImagePull(ctx context.Context, refStr string, options image.PullOptions) (io.ReadCloser, error) {
	return m.imagePullFn(ctx, refStr, options)
}

func (m *mockDockerClient) ContainerList(ctx context.Context, opts container.ListOptions) ([]container.Summary, error) {
	if m.containerListFn != nil {
		return m.containerListFn(ctx, opts)
	}
	return nil, nil
}
func (m *mockDockerClient) ContainerCreate(_ context.Context, _ *container.Config, _ *container.HostConfig, _ *network.NetworkingConfig, _ *ocispec.Platform, _ string) (container.CreateResponse, error) {
	return container.CreateResponse{}, nil
}
func (m *mockDockerClient) ContainerStart(_ context.Context, _ string, _ container.StartOptions) error {
	return nil
}
func (m *mockDockerClient) ContainerRemove(ctx context.Context, id string, opts container.RemoveOptions) error {
	if m.containerRemoveFn != nil {
		return m.containerRemoveFn(ctx, id, opts)
	}
	return nil
}
func (m *mockDockerClient) ContainerExecCreate(_ context.Context, _ string, _ container.ExecOptions) (container.ExecCreateResponse, error) {
	return container.ExecCreateResponse{}, nil
}
func (m *mockDockerClient) ContainerExecAttach(_ context.Context, _ string, _ container.ExecStartOptions) (dockertypes.HijackedResponse, error) {
	return dockertypes.HijackedResponse{}, nil
}
func (m *mockDockerClient) ContainerExecInspect(_ context.Context, _ string) (container.ExecInspect, error) {
	return container.ExecInspect{}, nil
}
func (m *mockDockerClient) CopyToContainer(_ context.Context, _, _ string, _ io.Reader, _ container.CopyToContainerOptions) error {
	return nil
}

func newTestExecutor(mock dockerClient) *DockerExecutor {
	return &DockerExecutor{
		cli:     mock,
		config:  &Config{Languages: map[Language]LangSettings{"python": {Image: "python:3.14-slim"}}},
		pools:   make(map[Language]chan string),
		errChan: make(chan error, 16),
	}
}

func TestEnsureImages_AlreadyPresent(t *testing.T) {
	pullCalled := false
	mock := &mockDockerClient{
		imageInspectFn: func(_ context.Context, _ string, _ ...client.ImageInspectOption) (image.InspectResponse, error) {
			return image.InspectResponse{}, nil // image exists
		},
		imagePullFn: func(_ context.Context, _ string, _ image.PullOptions) (io.ReadCloser, error) {
			pullCalled = true
			return nil, nil
		},
	}

	e := newTestExecutor(mock)
	err := e.ensureImages(context.Background())

	require.NoError(t, err)
	assert.False(t, pullCalled, "pull should not be called when image is already present")
}

func TestEnsureImages_PullOnMiss(t *testing.T) {
	pullCalled := false
	mock := &mockDockerClient{
		imageInspectFn: func(_ context.Context, _ string, _ ...client.ImageInspectOption) (image.InspectResponse, error) {
			return image.InspectResponse{}, cerrdefs.ErrNotFound
		},
		imagePullFn: func(_ context.Context, _ string, _ image.PullOptions) (io.ReadCloser, error) {
			pullCalled = true
			return io.NopCloser(strings.NewReader(`{"status":"Pull complete"}`)), nil
		},
	}

	e := newTestExecutor(mock)
	err := e.ensureImages(context.Background())

	require.NoError(t, err)
	assert.True(t, pullCalled, "pull should be called when image is missing")
}

func TestEnsureImages_PullError(t *testing.T) {
	mock := &mockDockerClient{
		imageInspectFn: func(_ context.Context, _ string, _ ...client.ImageInspectOption) (image.InspectResponse, error) {
			return image.InspectResponse{}, cerrdefs.ErrNotFound
		},
		imagePullFn: func(_ context.Context, _ string, _ image.PullOptions) (io.ReadCloser, error) {
			return nil, errors.New("registry unavailable")
		},
	}

	e := newTestExecutor(mock)
	err := e.ensureImages(context.Background())

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "registry unavailable")
}

func TestEnsureImages_InspectError(t *testing.T) {
	pullCalled := false
	mock := &mockDockerClient{
		imageInspectFn: func(_ context.Context, _ string, _ ...client.ImageInspectOption) (image.InspectResponse, error) {
			return image.InspectResponse{}, errors.New("permission denied")
		},
		imagePullFn: func(_ context.Context, _ string, _ image.PullOptions) (io.ReadCloser, error) {
			pullCalled = true
			return nil, nil
		},
	}

	e := newTestExecutor(mock)
	err := e.ensureImages(context.Background())

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "permission denied")
	assert.False(t, pullCalled, "pull should not be called on non-NotFound inspect error")
}
