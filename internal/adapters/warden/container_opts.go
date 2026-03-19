package warden

import (
	"path/filepath"

	"github.com/SecDuckOps/shared/scanner/ports"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
)

// scannerCacheVolumes maps scanner names to their cache directories inside containers.
// A Docker Named Volume ("duckops-cache-<scanner>") is mounted at each path
// so vulnerability databases and rule sets persist across ephemeral container runs.
var scannerCacheVolumes = map[string]string{
	"trivy":           "/root/.cache/trivy",
	"semgrep":         "/root/.semgrep",
	"grype":           "/root/.cache/grype",
	"gitleaks":        "/root/.cache/gitleaks",
	"trufflehog":      "/root/.trufflehog",
	"gosec":           "/root/.cache/gosec",
	"bandit":          "/root/.cache/bandit",
	"tfsec":           "/root/.cache/tfsec",
	"checkov":         "/root/.cache/checkov",
	"kics":            "/root/.cache/kics",
	"terrascan":       "/root/.cache/terrascan",
	"nuclei":          "/root/.config/nuclei",
	"osvscanner":      "/root/.cache/osv-scanner",
	"dependencycheck": "/usr/share/dependency-check/data",
	"brakeman":        "/root/.cache/brakeman",
	"tflint":          "/root/.tflint.d",
	"detectsecrets":   "/root/.cache/detect-secrets",
	"zap":             "/root/.ZAP",
}

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
		Tty:          false,
		AttachStdout: true,
		AttachStderr: true,
		WorkingDir:   "/scan/workspace",      // Enforced working directory inside container
	}

	hostCfg := &container.HostConfig{
		// NetworkMode: "none",          // Removed: Trivy and Semgrep MUST have network to fetch rules and DBs
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
				Target:   "/scan/workspace", // Target directory is ALWAYS read-only
				ReadOnly: true,
			},
		},
	}

	// Append persistent cache volume if scanner has a known cache path.
	// The Docker Named Volume ("duckops-cache-<scanner>") survives container
	// removal, so vulnerability DBs and rule caches are downloaded only once.
	if cachePath, ok := scannerCacheVolumes[opts.Scanner]; ok {
		hostCfg.Mounts = append(hostCfg.Mounts, mount.Mount{
			Type:   mount.TypeVolume,
			Source: "duckops-cache-" + opts.Scanner,
			Target: cachePath,
		})
	}

	netCfg := &network.NetworkingConfig{}

	return containerCfg, hostCfg, netCfg
}
