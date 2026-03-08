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
	imageInspectFn         func(ctx context.Context, imageID string, opts ...client.ImageInspectOption) (image.InspectResponse, error)
	imagePullFn            func(ctx context.Context, refStr string, options image.PullOptions) (io.ReadCloser, error)
	containerListFn        func(ctx context.Context, opts container.ListOptions) ([]container.Summary, error)
	containerRemoveFn      func(ctx context.Context, id string, opts container.RemoveOptions) error
	containerCreateFn      func(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, net *network.NetworkingConfig, platform *ocispec.Platform, name string) (container.CreateResponse, error)
	containerStartFn       func(ctx context.Context, id string, opts container.StartOptions) error
	containerExecCreateFn  func(ctx context.Context, id string, config container.ExecOptions) (container.ExecCreateResponse, error)
	containerExecAttachFn  func(ctx context.Context, execID string, config container.ExecStartOptions) (dockertypes.HijackedResponse, error)
	containerExecInspectFn func(ctx context.Context, execID string) (container.ExecInspect, error)
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
func (m *mockDockerClient) ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, net *network.NetworkingConfig, platform *ocispec.Platform, name string) (container.CreateResponse, error) {
	if m.containerCreateFn != nil {
		return m.containerCreateFn(ctx, config, hostConfig, net, platform, name)
	}
	return container.CreateResponse{ID: "test-container-id"}, nil
}
func (m *mockDockerClient) ContainerStart(ctx context.Context, id string, opts container.StartOptions) error {
	if m.containerStartFn != nil {
		return m.containerStartFn(ctx, id, opts)
	}
	return nil
}
func (m *mockDockerClient) ContainerRemove(ctx context.Context, id string, opts container.RemoveOptions) error {
	if m.containerRemoveFn != nil {
		return m.containerRemoveFn(ctx, id, opts)
	}
	return nil
}
func (m *mockDockerClient) ContainerExecCreate(ctx context.Context, id string, config container.ExecOptions) (container.ExecCreateResponse, error) {
	if m.containerExecCreateFn != nil {
		return m.containerExecCreateFn(ctx, id, config)
	}
	return container.ExecCreateResponse{ID: "test-exec-id"}, nil
}
func (m *mockDockerClient) ContainerExecAttach(ctx context.Context, execID string, config container.ExecStartOptions) (dockertypes.HijackedResponse, error) {
	if m.containerExecAttachFn != nil {
		return m.containerExecAttachFn(ctx, execID, config)
	}
	return dockertypes.HijackedResponse{}, nil
}
func (m *mockDockerClient) ContainerExecInspect(ctx context.Context, execID string) (container.ExecInspect, error) {
	if m.containerExecInspectFn != nil {
		return m.containerExecInspectFn(ctx, execID)
	}
	return container.ExecInspect{ExitCode: 0}, nil
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
