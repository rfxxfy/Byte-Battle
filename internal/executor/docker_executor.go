package executor

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"sync"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

const poolSize = 3

type DockerExecutor struct {
	cli   *client.Client
	config *Config
	pools  map[Language]chan string
	mu     sync.Mutex
}

func NewDockerExecutor(cfg *Config) (*DockerExecutor, error) {
	opts := []client.Opt{
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	}
	if cfg.DockerHost != "" {
		opts = append(opts, client.WithHost(cfg.DockerHost))
	}

	cli, err := client.NewClientWithOpts(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}

	e := &DockerExecutor{
		cli:    cli,
		config: cfg,
		pools:  make(map[Language]chan string),
	}
	
	e.initPools()
	return e, nil
}

func (e *DockerExecutor) initPools() {
	for lang, settings := range e.config.Languages {
		e.pools[lang] = make(chan string, poolSize)
		go e.maintainPool(lang, settings)
	}
}

func (e *DockerExecutor) maintainPool(lang Language, settings LangSettings) {
	for {
		if len(e.pools[lang]) < poolSize {
			id, err := e.createWarmContainer(context.Background(), settings)
			if err == nil {
				select {
				case e.pools[lang] <- id:
				default:
					// Pool full, cleanup
					e.cleanupContainer(context.Background(), id)
				}
			} else {
				time.Sleep(time.Second) // Backoff on error
			}
		} else {
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func (e *DockerExecutor) createWarmContainer(ctx context.Context, langConfig LangSettings) (string, error) {
	memLimit := langConfig.MemoryLimit
	if memLimit == 0 {
		memLimit = 100 * 1024 * 1024
	}

	nanoCPUs := int64(1000000000)
	pidsLimit := int64(64)

	hostConfig := &container.HostConfig{
		Resources: container.Resources{
			Memory:    memLimit,
			NanoCPUs:  nanoCPUs,
			PidsLimit: &pidsLimit,
		},
		NetworkMode: "none",
		CapDrop:     []string{"ALL"},
		Tmpfs: map[string]string{
			"/tmp": "",
			"/run": "",
		},
	}

	containerConfig := &container.Config{
		Image:      langConfig.Image,
		Cmd:        []string{"tail", "-f", "/dev/null"}, // Keep running
		Tty:        false,
		WorkingDir: "/app",
	}

	resp, err := e.cli.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, "")
	if err != nil {
		return "", err
	}
	
	if err := e.cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		e.cli.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true})
		return "", err
	}
	
	return resp.ID, nil
}

func (e *DockerExecutor) cleanupContainer(ctx context.Context, id string) {
	// Timeout for cleanup
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	e.cli.ContainerRemove(ctx, id, container.RemoveOptions{Force: true})
}

func (e *DockerExecutor) Run(ctx context.Context, req ExecutionRequest) (ExecutionResult, error) {
	langConfig, ok := e.config.Languages[req.Language]
	if !ok {
		return ExecutionResult{}, fmt.Errorf("unsupported language: %s", req.Language)
	}

	// Try to get from pool if default limits
	var containerID string
	var fromPool bool
	
	defaultMem := langConfig.MemoryLimit
	if defaultMem == 0 { defaultMem = 100 * 1024 * 1024 }
	
	reqMem := req.MemoryLimit
	if reqMem == 0 { reqMem = defaultMem }

	if reqMem == defaultMem {
		select {
		case containerID = <-e.pools[req.Language]:
			fromPool = true
		default:
			// Pool empty, create new
		}
	}

	if !fromPool {
		// Create ad-hoc container
		// We use the same createWarmContainer logic but maybe different limits if needed
		// For simplicity, reusing logic but we need to handle non-default limits here if we supported them fully.
		// Since createWarmContainer uses config limits, we might need a custom create if limits differ.
		// For MVP, assuming limits match or we use a separate create logic.
		
		// To support custom limits correctly:
		var err error
		containerID, err = e.createContainerWithLimits(ctx, langConfig, reqMem)
		if err != nil {
			return ExecutionResult{Error: err}, fmt.Errorf("failed to create container: %w", err)
		}
	}

	defer e.cleanupContainer(context.Background(), containerID)

	hasStdin := req.Stdin != ""
	files := map[string]string{
		langConfig.SourceFile: req.Code,
	}
	if hasStdin {
		files["input.txt"] = req.Stdin
	}

	if err := e.copyFilesToContainer(ctx, containerID, files); err != nil {
		return ExecutionResult{Error: err}, fmt.Errorf("failed to copy files: %w", err)
	}

	// Exec the code
	cmd := e.buildShellCommand(langConfig, hasStdin)
	
	execConfig := container.ExecOptions{
		Cmd:          []string{"/bin/sh", "-c", cmd},
		AttachStdout: true,
		AttachStderr: true,
	}
	
	execResp, err := e.cli.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		return ExecutionResult{Error: err}, fmt.Errorf("failed to create exec: %w", err)
	}

	attachResp, err := e.cli.ContainerExecAttach(ctx, execResp.ID, container.ExecStartOptions{})
	if err != nil {
		return ExecutionResult{Error: err}, fmt.Errorf("failed to attach exec: %w", err)
	}
	defer attachResp.Close()

	// Wait and Read
	limit := req.TimeLimit
	if limit == 0 && langConfig.TimeLimit > 0 {
		limit = time.Duration(langConfig.TimeLimit) * time.Second
	}
	if limit == 0 {
		limit = 5 * time.Second
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	outputDone := make(chan error, 1)

	go func() {
		_, err := stdcopy.StdCopy(&stdoutBuf, &stderrBuf, attachResp.Reader)
		outputDone <- err
	}()

	startTime := time.Now()
	var timedOut bool
	
	select {
	case err := <-outputDone:
		if err != nil {
			return ExecutionResult{Error: err}, fmt.Errorf("error reading output: %w", err)
		}
	case <-time.After(limit):
		timedOut = true
		// We can't easily kill just the exec, so we rely on container cleanup (defer)
	case <-ctx.Done():
		return ExecutionResult{Error: ctx.Err()}, ctx.Err()
	}
	
	duration := time.Since(startTime)

	// Get Exit Code
	inspect, err := e.cli.ContainerExecInspect(ctx, execResp.ID)
	exitCode := 0
	if err == nil {
		exitCode = inspect.ExitCode
	} else if !timedOut {
		// Log error but proceed?
	}
	
	if timedOut {
		exitCode = 124
	}

	const maxLogSize = 10 * 1024
	stdoutStr := stdoutBuf.String()
	if len(stdoutStr) > maxLogSize {
		stdoutStr = stdoutStr[:maxLogSize] + "...[truncated]"
	}
	stderrStr := stderrBuf.String()
	if len(stderrStr) > maxLogSize {
		stderrStr = stderrStr[:maxLogSize] + "...[truncated]"
	}

	res := ExecutionResult{
		Stdout:     stdoutStr,
		Stderr:     stderrStr,
		ExitCode:   exitCode,
		TimeUsed:   duration,
		MemoryUsed: 0,
	}

	if timedOut {
		res.Stderr += "\nExecution timed out."
	}

	return res, nil
}

// Helper to create ad-hoc container with specific limits
func (e *DockerExecutor) createContainerWithLimits(ctx context.Context, langConfig LangSettings, memLimit int64) (string, error) {
	// Similar to createWarmContainer but with specific memory
	nanoCPUs := int64(1000000000)
	pidsLimit := int64(64)

	hostConfig := &container.HostConfig{
		Resources: container.Resources{
			Memory:    memLimit,
			NanoCPUs:  nanoCPUs,
			PidsLimit: &pidsLimit,
		},
		NetworkMode: "none",
		CapDrop:     []string{"ALL"},
		Tmpfs: map[string]string{
			"/tmp": "",
			"/run": "",
		},
	}

	containerConfig := &container.Config{
		Image:      langConfig.Image,
		Cmd:        []string{"tail", "-f", "/dev/null"},
		Tty:        false,
		WorkingDir: "/app",
	}

	resp, err := e.cli.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, "")
	if err != nil {
		return "", err
	}
	
	if err := e.cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		e.cli.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true})
		return "", err
	}
	
	return resp.ID, nil
}


func (e *DockerExecutor) buildShellCommand(cfg LangSettings, hasStdin bool) string {
	var parts []string
	if len(cfg.CompileCmd) > 0 {
		parts = append(parts, strings.Join(cfg.CompileCmd, " "))
	}
	
	runCmd := strings.Join(cfg.RunCmd, " ")
	if hasStdin {
		runCmd += " < input.txt"
	}
	
	if len(parts) > 0 {
		parts = append(parts, runCmd)
		return strings.Join(parts, " && ")
	}
	return runCmd
}

func (e *DockerExecutor) copyFilesToContainer(ctx context.Context, containerID string, files map[string]string) error {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	for name, content := range files {
		hdr := &tar.Header{
			Name: name,
			Mode: 0644,
			Size: int64(len(content)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			return err
		}
	}
	
	if err := tw.Close(); err != nil {
		return err
	}

	return e.cli.CopyToContainer(ctx, containerID, "/app", &buf, container.CopyToContainerOptions{})
}
