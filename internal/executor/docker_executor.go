package executor

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

const (
	poolSize                = 3
	nanoCPUs                = int64(1000000000)
	pidsLimit               = int64(64)
	defaultMemoryLimit      = 100 * 1024 * 1024
	poolMaintainerMaxRounds = 1 << 20
	poolLabelKey            = "bytebattle"
	poolLabelVal            = "pool"
)

type DockerExecutor struct {
	cli     dockerClient
	config  *Config
	pools   map[Language]chan string
	errChan chan error
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
		cli:     cli,
		config:  cfg,
		pools:   make(map[Language]chan string),
		errChan: make(chan error, 16),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	e.cleanupPreviousPools(ctx)

	if err := e.ensureImages(ctx); err != nil {
		return nil, fmt.Errorf("failed to ensure executor images: %w", err)
	}
	e.initPools()
	go e.logPoolErrors()
	return e, nil
}

// cleanupPreviousPools removes any warm containers left over from a previous run
// (graceful shutdown race or crash). Called at startup before new pools are created.
func (e *DockerExecutor) cleanupPreviousPools(ctx context.Context) {
	f := filters.NewArgs(filters.Arg("label", poolLabelKey+"="+poolLabelVal))
	containers, err := e.cli.ContainerList(ctx, container.ListOptions{Filters: f, All: true})
	if err != nil {
		log.Printf("cleanup previous pools: %v", err)
		return
	}
	for i := range containers {
		e.cleanupContainer(ctx, containers[i].ID)
	}
	if len(containers) > 0 {
		log.Printf("cleaned up %d leftover pool containers", len(containers))
	}
}

func (e *DockerExecutor) ensureImages(ctx context.Context) error {
	var wg sync.WaitGroup
	errs := make(chan error, len(e.config.Languages))

	for _, settings := range e.config.Languages {
		wg.Add(1)
		go func(img string) {
			defer wg.Done()
			if _, err := e.cli.ImageInspect(ctx, img); err == nil {
				return // already present
			} else if !cerrdefs.IsNotFound(err) {
				errs <- fmt.Errorf("inspect %s: %w", img, err)
				return
			}
			log.Printf("pulling executor image %s...", img)
			rc, err := e.cli.ImagePull(ctx, img, image.PullOptions{})
			if err != nil {
				errs <- fmt.Errorf("pull %s: %w", img, err)
				return
			}
			defer rc.Close()
			if _, err := io.Copy(io.Discard, rc); err != nil {
				errs <- fmt.Errorf("pull %s: %w", img, err)
				return
			}
			log.Printf("pulled executor image %s", img)
		}(settings.Image)
	}

	wg.Wait()
	close(errs)

	var msgs []string
	for err := range errs {
		msgs = append(msgs, err.Error())
	}
	if len(msgs) > 0 {
		return fmt.Errorf("%s", strings.Join(msgs, "; "))
	}
	return nil
}

func (e *DockerExecutor) logPoolErrors() {
	for err := range e.errChan {
		if err != nil {
			log.Printf("executor pool error: %v", err)
		}
	}
}

func (e *DockerExecutor) initPools() {
	for lang, settings := range e.config.Languages {
		e.pools[lang] = make(chan string, poolSize)
		go e.maintainPool(lang, &settings)
	}
}

func (e *DockerExecutor) maintainPool(lang Language, settings *LangSettings) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigChan)
	go func() {
		<-sigChan
		cancel()
	}()

	for range poolMaintainerMaxRounds {
		if e.maintainPoolIteration(ctx, lang, settings) {
			break
		}
	}
	// best-effort drain on graceful shutdown; leftover containers are cleaned on next startup
	for {
		select {
		case id := <-e.pools[lang]:
			e.cleanupContainer(context.Background(), id)
		default:
			return
		}
	}
}

func (e *DockerExecutor) maintainPoolIteration(ctx context.Context, lang Language, settings *LangSettings) (exit bool) {
	if e.awaitOrDone(ctx, 100*time.Millisecond) {
		e.sendPoolErr(ctx.Err())
		return true
	}
	if len(e.pools[lang]) >= poolSize {
		return false
	}
	return e.tryFillPool(ctx, lang, settings)
}

func (e *DockerExecutor) awaitOrDone(ctx context.Context, d time.Duration) bool {
	select {
	case <-ctx.Done():
		return true
	case <-time.After(d):
		return false
	}
}

func (e *DockerExecutor) sendPoolErr(err error) {
	select {
	case e.errChan <- err:
	default:
	}
}

func (e *DockerExecutor) tryFillPool(ctx context.Context, lang Language, settings *LangSettings) (exit bool) {
	id, err := e.createWarmContainer(ctx, settings)
	if err != nil {
		return e.awaitOrDone(ctx, time.Second)
	}
	select {
	case e.pools[lang] <- id:
		return false
	case <-ctx.Done():
		e.cleanupContainer(context.Background(), id)
		e.sendPoolErr(ctx.Err())
		return true
	default:
		e.cleanupContainer(context.Background(), id)
		return false
	}
}

func (e *DockerExecutor) createWarmContainer(ctx context.Context, langConfig *LangSettings) (string, error) {
	memLimit := langConfig.MemoryLimit
	if memLimit == 0 {
		memLimit = defaultMemoryLimit
	}

	pidsLimitPtr := pidsLimit

	hostConfig := &container.HostConfig{
		Resources: container.Resources{
			Memory:    memLimit,
			NanoCPUs:  nanoCPUs,
			PidsLimit: &pidsLimitPtr,
		},
		NetworkMode: "none",
		CapDrop:     []string{"ALL"},
		Tmpfs: map[string]string{
			"/tmp": "exec",
			"/run": "",
		},
	}

	containerConfig := &container.Config{
		Image:      langConfig.Image,
		Cmd:        []string{"tail", "-f", "/dev/null"}, // Keep running
		Tty:        false,
		WorkingDir: "/app",
		Labels:     map[string]string{poolLabelKey: poolLabelVal},
	}

	resp, err := e.cli.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, "")
	if err != nil {
		return "", err
	}

	if err := e.cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		_ = e.cli.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true})
		return "", err
	}

	if langConfig.WarmupCmd != "" {
		if err := e.runWarmup(ctx, resp.ID, langConfig.WarmupCmd); err != nil {
			log.Printf("warmup failed for %s: %v", langConfig.Image, err)
		}
	}

	return resp.ID, nil
}

func (e *DockerExecutor) runWarmup(ctx context.Context, containerID, cmd string) error {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	execResp, err := e.cli.ContainerExecCreate(ctx, containerID, container.ExecOptions{
		Cmd: []string{"/bin/sh", "-c", cmd},
	})
	if err != nil {
		return err
	}
	attachResp, err := e.cli.ContainerExecAttach(ctx, execResp.ID, container.ExecStartOptions{})
	if err != nil {
		return err
	}
	defer attachResp.Close()
	go func() {
		<-ctx.Done()
		attachResp.Close()
	}()
	if _, err = io.Copy(io.Discard, attachResp.Reader); err != nil && ctx.Err() == nil {
		return err
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	insp, err := e.cli.ContainerExecInspect(ctx, execResp.ID)
	if err != nil {
		return err
	}
	if insp.ExitCode != 0 {
		return fmt.Errorf("warmup exited with code %d", insp.ExitCode)
	}
	return nil
}

func (e *DockerExecutor) cleanupContainer(ctx context.Context, id string) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_ = e.cli.ContainerRemove(ctx, id, container.RemoveOptions{Force: true})
}

func (e *DockerExecutor) Run(ctx context.Context, req ExecutionRequest) (ExecutionResult, error) {
	langConfig, ok := e.config.Languages[req.Language]
	if !ok {
		return ExecutionResult{}, fmt.Errorf("unsupported language: %s", req.Language)
	}

	containerID, err := e.getOrCreateContainer(ctx, req, &langConfig)
	if err != nil {
		return ExecutionResult{Error: err}, err
	}
	defer e.cleanupContainer(context.Background(), containerID)

	files := map[string]string{langConfig.SourceFile: req.Code}
	if req.Stdin != "" {
		files["input.txt"] = req.Stdin
	}
	if err := e.copyFilesToContainer(ctx, containerID, files); err != nil {
		return ExecutionResult{Error: err}, fmt.Errorf("failed to copy files: %w", err)
	}

	return e.execInContainer(ctx, containerID, &langConfig, req.Stdin != "", req.TimeLimit)
}

func (e *DockerExecutor) getOrCreateContainer(ctx context.Context, req ExecutionRequest, langConfig *LangSettings) (string, error) {
	defaultMem := langConfig.MemoryLimit
	if defaultMem == 0 {
		defaultMem = defaultMemoryLimit
	}
	reqMem := req.MemoryLimit
	if reqMem == 0 {
		reqMem = defaultMem
	}
	if reqMem == defaultMem {
		select {
		case id := <-e.pools[req.Language]:
			return id, nil
		default:
		}
	}
	return e.createContainerWithLimits(ctx, langConfig, reqMem)
}

func (e *DockerExecutor) execInContainer(ctx context.Context, containerID string, langConfig *LangSettings, hasStdin bool, timeLimit time.Duration) (ExecutionResult, error) {
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

	limit := timeLimit
	if limit == 0 && langConfig.TimeLimit > 0 {
		limit = time.Duration(langConfig.TimeLimit) * time.Second
	}
	if limit == 0 {
		limit = 5 * time.Second
	}

	stdoutBuf, stderrBuf := &bytes.Buffer{}, &bytes.Buffer{}
	outputDone := make(chan error, 1)
	go func() {
		_, err := stdcopy.StdCopy(stdoutBuf, stderrBuf, attachResp.Reader)
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
	case <-ctx.Done():
		return ExecutionResult{Error: ctx.Err()}, ctx.Err()
	}

	exitCode := 0
	if inspect, err := e.cli.ContainerExecInspect(ctx, execResp.ID); err == nil {
		exitCode = inspect.ExitCode
	}
	if timedOut {
		exitCode = 124
	}

	const maxLogSize = 10 * 1024
	res := ExecutionResult{
		Stdout:     truncateString(stdoutBuf.String(), maxLogSize),
		Stderr:     truncateString(stderrBuf.String(), maxLogSize),
		ExitCode:   exitCode,
		TimeUsed:   time.Since(startTime),
		MemoryUsed: 0,
	}
	if timedOut {
		res.Stderr += "\nExecution timed out."
	}
	return res, nil
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "...[truncated]"
}

// Helper to create ad-hoc container with specific limits
func (e *DockerExecutor) createContainerWithLimits(ctx context.Context, langConfig *LangSettings, memLimit int64) (string, error) {
	pidsLimitPtr := pidsLimit

	hostConfig := &container.HostConfig{
		Resources: container.Resources{
			Memory:    memLimit,
			NanoCPUs:  nanoCPUs,
			PidsLimit: &pidsLimitPtr,
		},
		NetworkMode: "none",
		CapDrop:     []string{"ALL"},
		Tmpfs: map[string]string{
			"/tmp": "exec",
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
		_ = e.cli.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true})
		return "", err
	}

	return resp.ID, nil
}

func (e *DockerExecutor) buildShellCommand(cfg *LangSettings, hasStdin bool) string {
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
			Mode: 0o644,
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
