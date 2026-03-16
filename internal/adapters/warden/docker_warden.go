package warden

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/SecDuckOps/shared/scanner/domain"
	"github.com/SecDuckOps/shared/scanner/ports"
	"github.com/SecDuckOps/shared/types"
	"github.com/docker/docker/api/types/container"
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

	// 3. Create Container
	resp, err := w.cli.ContainerCreate(ctx, containerCfg, hostCfg, netCfg, nil, "")
	if err != nil {
		return domain.ScanResult{}, types.Wrap(err, types.ErrCodeInternal, "failed to create scanner container")
	}

	containerID := resp.ID

	// Defer cleanup: FORCE REMOVE the container when we are done
	defer func() {
		_ = w.cli.ContainerRemove(context.Background(), containerID, container.RemoveOptions{Force: true})
	}()

	// 4. Start Container
	if err := w.cli.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		return domain.ScanResult{}, types.Wrap(err, types.ErrCodeInternal, "failed to start scanner container")
	}

	// 5. Wait for completion (with context)
	statusCh, errCh := w.cli.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)
	
	var statusCode int64 = -1
	select {
	case err := <-errCh:
		if err != nil {
			return domain.ScanResult{}, types.Wrap(err, types.ErrCodeInternal, "container execution failed")
		}
	case status := <-statusCh:
		statusCode = status.StatusCode
	case <-ctx.Done():
		return domain.ScanResult{}, types.Wrap(ctx.Err(), types.ErrCodeExecutionFailed, "scan timeout or context cancelled")
	}

	// 6. Capture Logs (stdout and stderr)
	out, err := w.cli.ContainerLogs(ctx, containerID, container.LogsOptions{ShowStdout: true, ShowStderr: true})
	if err != nil {
		return domain.ScanResult{}, types.Wrap(err, types.ErrCodeInternal, "failed to read container logs")
	}
	defer out.Close()

	var stdoutBuf, stderrBuf bytes.Buffer
	// Docker multiplexes stdout/stderr, use stdcopy to demux
	_, err = stdcopy.StdCopy(&stdoutBuf, &stderrBuf, out)
	if err != nil {
		// Fallback for non-TTY just reading it all if stdcopy fails
		buf, _ := io.ReadAll(out)
		stdoutBuf.Write(buf)
	}

	rawOutput := stdoutBuf.String()
	errMsg := stderrBuf.String()
	
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
