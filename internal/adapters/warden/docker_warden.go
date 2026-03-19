package warden

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/SecDuckOps/shared/scanner/domain"
	"github.com/SecDuckOps/shared/scanner/ports"
	"github.com/SecDuckOps/shared/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

// DockerWarden adapter executes scanner processes securely in Docker
type DockerWarden struct {
	cli *client.Client
}

// NewDockerWarden creates a new DockerWarden adapter
func NewDockerWarden() (*DockerWarden, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, types.Wrap(err, types.ErrCodeInternal, "failed to initialize docker client")
	}
	return &DockerWarden{cli: cli}, nil
}

// Ensure interface compliance
var _ ports.ScannerPort = (*DockerWarden)(nil)

// RunScan executes the scanner securely within a Docker container.
func (w *DockerWarden) RunScan(ctx context.Context, opts ports.ScanOpts) (domain.ScanResult, error) {
	start := time.Now()

	resolvedImage, err := GetImageForScanner(opts.Scanner)
	if err != nil {
		// Use provided image if not found in registry (custom scanner fallback)
		if opts.Image != "" {
			resolvedImage = opts.Image
		} else {
			return domain.ScanResult{}, types.Wrap(err, types.ErrCodeInvalidInput, "scanner not supported and no fallback image provided")
		}
	}

	// 1. Ensure image exists locally or pull it
	err = w.ensureImage(ctx, resolvedImage)
	if err != nil {
		return domain.ScanResult{}, types.Wrapf(err, types.ErrCodeInternal, "failed to ensure image %s", resolvedImage)
	}

	// 2. Build secure container config
	containerCfg, hostCfg, netCfg := buildContainerConfig(opts, resolvedImage)
	
	// Tie this container to the current execution session
	if containerCfg.Labels == nil {
		containerCfg.Labels = make(map[string]string)
	}
	containerCfg.Labels["duckops.managed"] = "true" // Always mark as managed by DuckOps

	sessionID, _ := ctx.Value("sessionID").(string)
	if sessionID != "" {
		containerCfg.Labels["duckops.session.id"] = sessionID
	}

	// 3. Create Container
	resp, err := w.cli.ContainerCreate(ctx, containerCfg, hostCfg, netCfg, nil, "")
	if err != nil {
		return domain.ScanResult{}, types.Wrap(err, types.ErrCodeInternal, "failed to create scanner container")
	}

	containerID := resp.ID

	// Helper to force-remove the container (used on early-exit errors and after output capture).
	removeContainer := func() {
		_ = w.cli.ContainerRemove(context.Background(), containerID, container.RemoveOptions{Force: true})
	}

	// 4. Start Container
	if err := w.cli.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		removeContainer()
		return domain.ScanResult{}, types.Wrap(err, types.ErrCodeInternal, "failed to start scanner container")
	}

	// 5. Wait for completion (with context)
	statusCh, errCh := w.cli.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)
	
	var statusCode int64 = -1
	select {
	case err := <-errCh:
		if err != nil {
			removeContainer()
			return domain.ScanResult{}, types.Wrap(err, types.ErrCodeInternal, "container execution failed")
		}
	case status := <-statusCh:
		statusCode = status.StatusCode
	case <-ctx.Done():
		removeContainer()
		return domain.ScanResult{}, types.Wrap(ctx.Err(), types.ErrCodeExecutionFailed, "scan timeout or context cancelled")
	}

	// 6. Capture Logs (stdout and stderr) — container is still alive at this point
	out, err := w.cli.ContainerLogs(ctx, containerID, container.LogsOptions{ShowStdout: true, ShowStderr: true})
	if err != nil {
		removeContainer()
		return domain.ScanResult{}, types.Wrap(err, types.ErrCodeInternal, "failed to read container logs")
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	// Docker multiplexes stdout/stderr, use stdcopy to demux
	_, err = stdcopy.StdCopy(&stdoutBuf, &stderrBuf, out)
	if err != nil {
		// Fallback for non-TTY just reading it all if stdcopy fails
		buf, _ := io.ReadAll(out)
		stdoutBuf.Write(buf)
	}
	out.Close()

	rawOutput := stdoutBuf.String()
	errMsg := stderrBuf.String()

	// 7. WE NO LONGER REMOVE THE CONTAINER HERE!
	// The user requested that containers bind to the session lifecycle and remain active/persisted
	// until the session itself is closed or deleted.
	
	duration := time.Since(start).String()

	// If container returned non-zero, capture it as error in result but STILL pass back output
	// Many scanners return non-zero if vulnerabilities are found!
	if statusCode != 0 && statusCode != -1 {
		if errMsg == "" {
			errMsg = fmt.Sprintf("Scanner exited with code %d", statusCode)
		} else {
			errMsg = fmt.Sprintf("Exit %d: %s", statusCode, errMsg)
		}
	}

	// We return a raw ScanResult. Note that the findings slice is empty here 
	// because parsing happens upstream via the ResultParserPort!
	res := domain.ScanResult{
		ScanID:      fmt.Sprintf("scan-%d", time.Now().UnixNano()),
		ScannerName: opts.Scanner,
		Target:      opts.TargetDir,
		StartTime:   start,
		EndTime:     time.Now(),
		Duration:    duration,
		Error:       errMsg,
		Findings:    nil, // To be populated by Aggregator/Parser
	}
	
	// Store raw output correctly in the struct natively rather than overriding Target
	res.RawOutput = rawOutput

	return res, nil
}

// HealthCheck verifies connectivity to the Docker daemon.
func (w *DockerWarden) HealthCheck(ctx context.Context) error {
	ping, err := w.cli.Ping(ctx)
	if err != nil {
		return types.Wrap(err, types.ErrCodeInternal, "docker daemon is not reachable")
	}
	if ping.APIVersion == "" {
		return types.New(types.ErrCodeInternal, "empty api version from docker ping")
	}
	return nil
}

// WarmupImages pulls all scanner images in parallel at agent startup.
// Failures are non-fatal — the scan will attempt a pull at runtime if needed.
// Satisfies the ScannerPort interface.
func (w *DockerWarden) WarmupImages(ctx context.Context, scannerNames []string) error {
	var wg sync.WaitGroup
	for _, name := range scannerNames {
		name := name
		wg.Add(1)
		go func() {
			defer wg.Done()
			imageRef, err := GetImageForScanner(name)
			if err != nil {
				return // unknown scanner — skip silently
			}
			if err := w.ensureImage(ctx, imageRef); err != nil {
				// non-fatal: scan will retry at runtime
				return
			}
		}()
	}
	wg.Wait()
	return nil
}

// ensureImage checks if the image exists or pulls it.
func (w *DockerWarden) ensureImage(ctx context.Context, imageRef string) error {
	_, _, err := w.cli.ImageInspectWithRaw(ctx, imageRef)
	if err == nil {
		return nil
	}
	rc, err := w.cli.ImagePull(ctx, imageRef, image.PullOptions{})
	if err != nil {
		return types.Wrapf(err, types.ErrCodeInternal, "failed to pull image %s", imageRef)
	}
	defer rc.Close()
	_, _ = io.Copy(io.Discard, rc)

	// B4 Fix: Verify the image actually exists now instead of string-matching JSON logs
	_, _, err = w.cli.ImageInspectWithRaw(ctx, imageRef)
	if err != nil {
		return types.Wrapf(err, types.ErrCodeInternal, "failed to verify image %s after pull", imageRef)
	}

	return nil
}

// CleanupSession removes all scanner containers associated with the given session ID.
func (w *DockerWarden) CleanupSession(ctx context.Context, sessionID string) error {
	if sessionID == "" {
		return nil
	}

	// Filter containers by label duckops.session.id
	f := filters.NewArgs()
	f.Add("label", fmt.Sprintf("duckops.session.id=%s", sessionID))

	containers, err := w.cli.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: f,
	})
	if err != nil {
		return types.Wrapf(err, types.ErrCodeInternal, "failed to list containers for session cleanup: %s", sessionID)
	}

	for _, c := range containers {
		_ = w.cli.ContainerRemove(ctx, c.ID, container.RemoveOptions{
			Force: true,
		})
	}

	return nil
}

// CleanupAllManagedContainers removes any and all containers created by this DuckOps instance.
func (w *DockerWarden) CleanupAllManagedContainers(ctx context.Context) error {
	f := filters.NewArgs()
	f.Add("label", "duckops.managed=true")

	containers, err := w.cli.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: f,
	})
	if err != nil {
		return types.Wrap(err, types.ErrCodeInternal, "failed to list all managed containers for cleanup")
	}

	for _, c := range containers {
		_ = w.cli.ContainerRemove(ctx, c.ID, container.RemoveOptions{
			Force: true,
		})
	}

	return nil
}
