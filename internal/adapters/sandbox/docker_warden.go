package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/SecDuckOps/agent/internal/domain/security"
	"github.com/SecDuckOps/agent/internal/ports"
	"github.com/SecDuckOps/shared/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

// DockerWarden is the production-grade implementation of the Warden port.
type DockerWarden struct {
	cli *client.Client
}

// NewDockerWarden creates a new DockerWarden.
func NewDockerWarden() (*DockerWarden, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, types.Wrap(err, types.ErrCodeInternal, "failed to create docker client")
	}
	return &DockerWarden{cli: cli}, nil
}

// Run executes a scan in an isolated, secure container.
func (w *DockerWarden) Run(ctx context.Context, spec security.ScanSpec) (security.ScanResult, error) {
	start := time.Now()
	result := security.ScanResult{
		Tool: spec.ToolName,
	}

	// 1. Prepare Environment Variables
	env := make([]string, 0, len(spec.EnvVars))
	for k, v := range spec.EnvVars {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	// 2. Configure Container (Full Security Constraints)
	config := &container.Config{
		Image:        spec.Image,
		Cmd:          spec.Command,
		Env:          env,
		Tty:          false,
		AttachStdout: true,
		AttachStderr: true,
		WorkingDir:   "/src",
		User:         "1000:1000", // Non-root execution
	}

	hostConfig := &container.HostConfig{
		// 🛡️ Security Isolation
		Mounts: []mount.Mount{
			{
				Type:     mount.TypeBind,
				Source:   spec.TargetPath,
				Target:   "/src",
				ReadOnly: true, // Read-only project directory mount
			},
		},
		ReadonlyRootfs: true, // No writing to root filesystem
		NetworkMode:    "none", // Isolated from network
		Resources: container.Resources{
			CPUQuota: spec.CPUQuota,
			Memory:   spec.MemoryLimit,
		},
		AutoRemove: true, // Clean up automatically
		CapDrop:    []string{"ALL"}, // Minimal capabilities
	}

	// 3. Ensure image availability
	if err := w.ensureImage(ctx, spec.Image); err != nil {
		return result, err
	}

	// 4. Create and Start Container
	resp, err := w.cli.ContainerCreate(ctx, config, hostConfig, nil, nil, "")
	if err != nil {
		return result, types.Wrap(err, types.ErrCodeInternal, "failed to create container")
	}

	if err := w.cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return result, types.Wrap(err, types.ErrCodeInternal, "failed to start container")
	}

	// 5. Capture Output Streams
	logs, err := w.cli.ContainerLogs(ctx, resp.ID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
	})
	if err != nil {
		return result, types.Wrap(err, types.ErrCodeInternal, "failed to capture logs")
	}
	defer logs.Close()

	var outputBuf bytes.Buffer
	outputDone := make(chan struct{})
	go func() {
		_, _ = stdcopy.StdCopy(&outputBuf, &outputBuf, logs)
		close(outputDone)
	}()

	// 6. Wait for Termination (Enforce Timeout)
	waitCtx, cancel := context.WithTimeout(ctx, spec.Timeout)
	defer cancel()

	statusCh, errCh := w.cli.ContainerWait(waitCtx, resp.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return result, types.Wrap(err, types.ErrCodeInternal, "wait failed")
		}
	case status := <-statusCh:
		result.ExitCode = int(status.StatusCode)
	case <-waitCtx.Done():
		_ = w.cli.ContainerKill(ctx, resp.ID, "SIGKILL")
		return result, types.New(types.ErrCodeExecutionFailed, "execution timeout")
	}

	<-outputDone
	result.RawOutput = outputBuf.Bytes()
	result.Duration = time.Since(start)

	return result, nil
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
	return nil
}

// Close releases the Docker client resources.
func (w *DockerWarden) Close() error {
	return w.cli.Close()
}

var _ ports.Warden = (*DockerWarden)(nil)
