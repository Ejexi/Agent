package warden

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/SecDuckOps/shared/ports"
)

var startupMu sync.Mutex

// AutoStartDocker attempts to launch the Docker daemon/desktop based on the OS.
// It explicitly guards against concurrent startups, uses detached processes,
// and polls with exponential backoff.
func AutoStartDocker(ctx context.Context, logger ports.Logger, dw *DockerWarden) error {
	// 1. Concurrency Guard
	if !startupMu.TryLock() {
		logger.Debug(ctx, "Skipping auto-start constraint: another goroutine is already initializing Docker.")
		return nil
	}
	defer startupMu.Unlock()

	logger.Info(ctx, "Attempting to auto-start Docker daemon...", ports.Field{Key: "os", Value: runtime.GOOS})

	cmd, err := buildStartCommand(logger)
	if err != nil {
		return fmt.Errorf("failed to build start command: %w", err)
	}

	startAt := time.Now()
	if err := cmd.Start(); err != nil { // Use Start() to avoid blocking indefinitely, we let the process detach
		return fmt.Errorf("failed to execute start command [%s]: %w", cmd.Path, err)
	}

	logger.Info(ctx, "Startup command issued successfully. Detaching and waiting for readiness.",
		ports.Field{Key: "command", Value: cmd.Path},
		ports.Field{Key: "args", Value: strings.Join(cmd.Args, " ")},
	)

	// In detached mode, we don't necessarily call cmd.Wait(). The external daemon will bootstrap itself.

	// 3. Readiness Probe with Exponential Backoff
	timeout := time.After(90 * time.Second)
	backoff := 500 * time.Millisecond
	maxBackoff := 5 * time.Second
	retryCount := 0

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			return fmt.Errorf("global timeout (90s) reached while waiting for Docker daemon to become healthy")
		default:
			retryCount++
			
			// If Warden is prematurely killed or disconnected
			if dw == nil || dw.cli == nil {
				return fmt.Errorf("docker client is nil during readiness polling")
			}

			// Perform strict health check
			err := dw.HealthCheck(ctx)
			if err == nil {
				latency := time.Since(startAt)
				logger.Info(ctx, "Docker daemon is running and healthy!",
					ports.Field{Key: "latency_ms", Value: latency.Milliseconds()},
					ports.Field{Key: "retries", Value: retryCount},
				)
				return nil
			}

			// Log explicitly transient issues without spamming
			if !strings.Contains(err.Error(), "Is the docker daemon running") && retryCount%5 == 0 {
				logger.Debug(ctx, "Probe failed, retrying...", ports.Field{Key: "error", Value: err.Error()})
			}

			// Apply exponential backoff delay
			time.Sleep(backoff)
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}
}

// buildStartCommand constructs OS-specific startup and appliesSysProcAttr for safe detachment.
func buildStartCommand(logger ports.Logger) (*exec.Cmd, error) {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		dockerPath := findDockerDesktopWindows()
		if dockerPath == "" {
			return nil, fmt.Errorf("could not locate Docker Desktop executable")
		}
		// Windows: Launch the EXE directly
		cmd = exec.Command(dockerPath)
		applySysProcAttr(cmd)

	case "darwin":
		cmd = exec.Command("open", "-a", "Docker")

	case "linux":
		if _, err := exec.LookPath("systemctl"); err == nil {
			cmd = exec.Command("systemctl", "start", "docker")
		} else if _, err := exec.LookPath("service"); err == nil {
			cmd = exec.Command("service", "docker", "start")
		} else {
			return nil, fmt.Errorf("neither 'systemctl' nor 'service' found on Linux")
		}
		applySysProcAttr(cmd)

	default:
		return nil, fmt.Errorf("auto-start not supported on OS: %s", runtime.GOOS)
	}

	return cmd, nil
}

// findDockerDesktopWindows safely probes the Registry mapping or well-known paths.
func findDockerDesktopWindows() string {
	fallbacks := []string{
		"C:\\Program Files\\Docker\\Docker\\Docker Desktop.exe",
		"D:\\Program Files\\Docker\\Docker\\Docker Desktop.exe",
		// Support alternative volume mappings
		"E:\\Program Files\\Docker\\Docker\\Docker Desktop.exe",
	}

	// 1. Registry query
	out, err := exec.Command("reg", "query", "HKLM\\SOFTWARE\\Docker Inc.\\Docker\\1.0", "/v", "AppPath").Output()
	if err == nil {
		lines := strings.Split(string(out), "\n")
		for _, line := range lines {
			if strings.Contains(line, "AppPath") {
				parts := strings.SplitN(line, "REG_SZ", 2)
				if len(parts) == 2 {
					return strings.TrimSpace(parts[1])
				}
			}
		}
	}

	// 2. Bruteforce os.Stat for absolute paths
	for _, path := range fallbacks {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}
