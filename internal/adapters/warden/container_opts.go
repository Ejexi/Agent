package warden

import (
	"path/filepath"

	"github.com/SecDuckOps/shared/scanner/ports"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
)

// buildContainerConfig constructs the secure container and host config
func buildContainerConfig(opts ports.ScanOpts, resolvedImage string) (*container.Config, *container.HostConfig, *network.NetworkingConfig) {

	// Commands logic. If empty, the image's default entrypoint handles it.
	var cmd []string
	if len(opts.Cmd) > 0 {
		cmd = opts.Cmd
	}

	targetAbs, _ := filepath.Abs(opts.TargetDir)

	containerCfg := &container.Config{
		Image:        resolvedImage,
		Cmd:          cmd,
		Env:          opts.Env,
		User:         "65534:65534",     // nobody:nogroup - enforce non-root execution
		Tty:          false,
		AttachStdout: true,
		AttachStderr: true,
		WorkingDir:   "/scan/workspace",      // Enforced working directory inside container
	}

	hostCfg := &container.HostConfig{
		NetworkMode: "none",          // No network access strictly enforced
		Privileged:  false,           // Never run privileged
		CapDrop:     []string{"ALL"}, // Drop all Linux capabilities
		SecurityOpt: []string{"no-new-privileges:true"}, // Prevent privilege escalation

		// Resource limitations
		Resources: container.Resources{
			Memory:   512 * 1024 * 1024, // 512 MB memory limit
			CPUQuota: 50000,             // 50% CPU limit
			PidsLimit: func() *int64 {
				p := int64(100)
				return &p
			}(),
		},

		// Mount logic
		Mounts: []mount.Mount{
			{
				Type:     mount.TypeBind,
				Source:   targetAbs,
				Target:   "/scan/workspace", // Updated: Target directory is ALWAYS read-only
				ReadOnly: true,              // Target directory is ALWAYS read-only
			},
		},
	}

	netCfg := &network.NetworkingConfig{}

	return containerCfg, hostCfg, netCfg
}
