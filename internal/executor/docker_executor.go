package executor

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"bytebattle/internal/apierr"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

const (
	poolSize           = 3
	queueMultiplier    = 4
	nanoCPUs           = int64(1000000000)
	pidsLimit          = int64(64)
	defaultMemoryLimit = 100 * 1024 * 1024
	poolLabelKey       = "bytebattle"
	poolLabelVal       = "pool"
)

type workItem struct {
	ctx    context.Context
	req    ExecutionRequest
	result chan<- workResult // buffered(1); worker writes exactly once
}

type workResult struct {
	res ExecutionResult
	err error
}

type langPool struct {
	queue    chan workItem
	settings LangSettings
}

type DockerExecutor struct {
	cli           dockerClient
	config        *Config
	pools         map[Language]*langPool
	errChan       chan error
	primedPerLang map[Language]*atomic.Bool
	primedCount   atomic.Int32
	ready         atomic.Bool
	shutdownCtx   context.Context
	shutdown      context.CancelFunc
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

	shutdownCtx, shutdown := context.WithCancel(context.Background())

	e := &DockerExecutor{
		cli:         cli,
		config:      cfg,
		pools:       make(map[Language]*langPool),
		errChan:     make(chan error, 16),
		shutdownCtx: shutdownCtx,
		shutdown:    shutdown,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	e.cleanupPreviousPools(ctx)

	if err := e.ensureImages(ctx); err != nil {
		return nil, fmt.Errorf("failed to ensure executor images: %w", err)
	}
	e.initPools()
	go e.logPoolErrors()
	go e.watchSignals()
	return e, nil
}

func (e *DockerExecutor) watchSignals() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigChan)
	select {
	case <-sigChan:
		e.shutdown()
	case <-e.shutdownCtx.Done():
	}
}

// cleanupPreviousPools removes warm containers left over from a previous crash or unclean shutdown.
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
				return
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

func (e *DockerExecutor) IsReady() bool {
	return len(e.pools) > 0 && e.ready.Load()
}

func (e *DockerExecutor) notifyPoolPrimed(lang Language) {
	if e.primedPerLang[lang].CompareAndSwap(false, true) {
		if e.primedCount.Add(1) == int32(len(e.pools)) {
			e.ready.Store(true)
		}
	}
}

func (e *DockerExecutor) initPools() {
	e.primedPerLang = make(map[Language]*atomic.Bool, len(e.config.Languages))
	for lang, settings := range e.config.Languages {
		size := settings.PoolSize
		if size <= 0 {
			size = poolSize
		}
		lp := &langPool{
			queue:    make(chan workItem, size*queueMultiplier),
			settings: settings,
		}
		e.pools[lang] = lp
		e.primedPerLang[lang] = new(atomic.Bool)

		for range size {
			go e.runWorker(lang, lp)
		}
	}
}

func (e *DockerExecutor) runWorker(lang Language, lp *langPool) {
	var containerID string
	primed := false

	for {
		if containerID == "" {
			id, ok := e.acquireContainer(lang, lp, &primed)
			if !ok {
				return
			}
			containerID = id
		}

		alive, ok := e.processNextItem(containerID, lp)
		if !ok {
			return
		}
		if !alive {
			e.cleanupContainer(context.Background(), containerID)
			containerID = ""
		}
	}
}

// acquireContainer returns (id, true) on success, ("", false) on shutdown.
func (e *DockerExecutor) acquireContainer(lang Language, lp *langPool, primed *bool) (string, bool) {
	for {
		id, err := e.createWarmContainer(context.Background(), &lp.settings)
		if err != nil {
			e.sendPoolErr(fmt.Errorf("worker %s: create container: %w", lang, err))
			select {
			case <-e.shutdownCtx.Done():
				return "", false
			case <-time.After(time.Second):
				continue
			}
		}
		if !*primed {
			e.notifyPoolPrimed(lang)
			*primed = true
		}
		return id, true
	}
}

// processNextItem returns (containerAlive, workerShouldContinue).
func (e *DockerExecutor) processNextItem(containerID string, lp *langPool) (alive, ok bool) {
	select {
	case item := <-lp.queue:
		res, err := e.executeWorkItem(item.ctx, containerID, item.req, &lp.settings)
		item.result <- workResult{res: res, err: err}

		cleanErr := e.cleanWorkDir(context.Background(), containerID)
		return cleanErr == nil && e.isContainerRunning(context.Background(), containerID), true

	case <-e.shutdownCtx.Done():
		e.drainQueueWithShutdown(lp)
		e.cleanupContainer(context.Background(), containerID)
		return false, false
	}
}

func (e *DockerExecutor) drainQueueWithShutdown(lp *langPool) {
	for {
		select {
		case item := <-lp.queue:
			item.result <- workResult{err: errors.New("executor is shutting down")}
		default:
			return
		}
	}
}

// Run enqueues req for execution. req.MemoryLimit is ignored, pool containers use the configured LangSettings.MemoryLimit.
func (e *DockerExecutor) Run(ctx context.Context, req ExecutionRequest) (ExecutionResult, error) {
	lp, ok := e.pools[req.Language]
	if !ok {
		return ExecutionResult{}, fmt.Errorf("unsupported language: %s", req.Language)
	}

	select {
	case <-e.shutdownCtx.Done():
		return ExecutionResult{}, errors.New("executor is shutting down")
	default:
	}

	resultCh := make(chan workResult, 1)
	item := workItem{ctx: ctx, req: req, result: resultCh}

	select {
	case lp.queue <- item:
	default:
		return ExecutionResult{}, apierr.New(apierr.ErrExecutorOverloaded, "executor queue is full, try again later")
	}

	select {
	case res := <-resultCh:
		return res.res, res.err
	case <-ctx.Done():
		return ExecutionResult{Error: ctx.Err()}, ctx.Err()
	}
}

func (e *DockerExecutor) executeWorkItem(ctx context.Context, containerID string, req ExecutionRequest, langConfig *LangSettings) (ExecutionResult, error) {
	files := map[string]string{langConfig.SourceFile: req.Code}
	if req.Stdin != "" {
		files["input.txt"] = req.Stdin
	}
	if err := e.copyFilesToContainer(ctx, containerID, files); err != nil {
		return ExecutionResult{Error: err}, fmt.Errorf("failed to copy files: %w", err)
	}
	return e.execInContainer(ctx, containerID, langConfig, req.Stdin != "", req.TimeLimit)
}

// cleanWorkDir removes user-written files from /app and /tmp so the container
// can be safely reused for the next request.
func (e *DockerExecutor) cleanWorkDir(ctx context.Context, containerID string) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	execResp, err := e.cli.ContainerExecCreate(ctx, containerID, container.ExecOptions{
		Cmd: []string{"/bin/sh", "-c", "rm -rf /app/* /tmp/*"},
	})
	if err != nil {
		return err
	}

	attachResp, err := e.cli.ContainerExecAttach(ctx, execResp.ID, container.ExecStartOptions{})
	if err != nil {
		return err
	}
	defer attachResp.Close()

	if deadline, ok := ctx.Deadline(); ok {
		_ = attachResp.Conn.SetDeadline(deadline)
	}
	go func() {
		<-ctx.Done()
		attachResp.Close()
	}()

	if _, err = io.Copy(io.Discard, attachResp.Reader); err != nil && ctx.Err() == nil {
		return err
	}
	return ctx.Err()
}

func (e *DockerExecutor) cleanupContainer(ctx context.Context, id string) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_ = e.cli.ContainerRemove(ctx, id, container.RemoveOptions{Force: true})
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
		SecurityOpt: []string{"no-new-privileges:true"},
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
	// SetDeadline lets io.Copy unblock exactly when the context expires,
	// without relying on goroutine scheduling. The goroutine below is kept
	// as a cleanup path for contexts cancelled without a deadline.
	if deadline, ok := ctx.Deadline(); ok {
		_ = attachResp.Conn.SetDeadline(deadline)
	}
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

func (e *DockerExecutor) execInContainer(ctx context.Context, containerID string, langConfig *LangSettings, hasStdin bool, timeLimit time.Duration) (ExecutionResult, error) {
	limit := timeLimit
	if limit == 0 && langConfig.TimeLimit > 0 {
		limit = time.Duration(langConfig.TimeLimit) * time.Second
	}
	if limit == 0 {
		limit = 5 * time.Second
	}

	cmd := e.buildShellCommand(langConfig, hasStdin, limit)
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

	stdoutBuf, stderrBuf := &bytes.Buffer{}, &bytes.Buffer{}
	outputDone := make(chan error, 1)
	go func() {
		_, err := stdcopy.StdCopy(stdoutBuf, stderrBuf, attachResp.Reader)
		outputDone <- err
	}()

	// The run command is already wrapped with timeout(1), so the process will
	// self-terminate and close the output stream when the limit is reached.
	// The Go-level timer is a safety net for cases where timeout(1) is
	// unavailable or the container is unresponsive.
	safetyNet := limit + 30*time.Second
	startTime := time.Now()
	var safetyFired bool

	select {
	case err := <-outputDone:
		if err != nil {
			return ExecutionResult{Error: err}, fmt.Errorf("error reading output: %w", err)
		}
	case <-time.After(safetyNet):
		safetyFired = true
	case <-ctx.Done():
		return ExecutionResult{Error: ctx.Err()}, ctx.Err()
	}

	exitCode := 0
	if inspect, err := e.cli.ContainerExecInspect(ctx, execResp.ID); err == nil {
		exitCode = inspect.ExitCode
	}

	timedOut := safetyFired || exitCode == 124

	const maxLogSize = 10 * 1024
	res := ExecutionResult{
		Stdout:   truncateString(stdoutBuf.String(), maxLogSize),
		Stderr:   truncateString(stderrBuf.String(), maxLogSize),
		ExitCode: exitCode,
		TimeUsed: time.Since(startTime),
	}
	if timedOut {
		res.ExitCode = 124
		res.Stderr += "\nExecution timed out."
	}
	return res, nil
}

func (e *DockerExecutor) buildShellCommand(cfg *LangSettings, hasStdin bool, timeLimit time.Duration) string {
	var parts []string
	if len(cfg.CompileCmd) > 0 {
		parts = append(parts, strings.Join(cfg.CompileCmd, " "))
	}

	runCmd := strings.Join(cfg.RunCmd, " ")
	if hasStdin {
		runCmd += " < input.txt"
	}

	secs := int(timeLimit.Seconds())
	if secs > 0 {
		runCmd = fmt.Sprintf("timeout %ds %s", secs, runCmd)
	}

	if len(parts) > 0 {
		parts = append(parts, runCmd)
		return strings.Join(parts, " && ")
	}
	return runCmd
}

func (e *DockerExecutor) isContainerRunning(ctx context.Context, id string) bool {
	info, err := e.cli.ContainerInspect(ctx, id)
	if err != nil {
		return false
	}
	return info.ContainerJSONBase != nil && info.State != nil && info.State.Running
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

func (e *DockerExecutor) sendPoolErr(err error) {
	select {
	case e.errChan <- err:
	default:
	}
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "...[truncated]"
}
