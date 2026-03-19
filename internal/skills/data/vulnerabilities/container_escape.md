---
name: container-escape
description: Container breakout techniques — privileged containers, volume mounts, capabilities, kernel exploits
---

# Container Escape

Container escapes allow an attacker to break out of the container namespace onto the host. Priority target in any containerised deployment.

## Common Escape Vectors

### Privileged Container
```bash
# Detection
docker inspect <id> | grep '"Privileged": true'

# Exploit (mount host filesystem)
mkdir /tmp/host
mount /dev/sda1 /tmp/host
chroot /tmp/host
```

### Dangerous Capabilities
| Capability | Escape Path |
|-----------|-------------|
| `CAP_SYS_ADMIN` | Mount host FS, nsenter, cgroup release_agent |
| `CAP_NET_ADMIN` | ARP spoofing, traffic interception |
| `CAP_SYS_PTRACE` | Attach to host PID namespace processes |
| `CAP_DAC_READ_SEARCH` | Read arbitrary host files via open_by_handle_at |
| `CAP_SYS_MODULE` | Load kernel modules |

### Writable Host Path Mounts
```yaml
# ⚠ Dangerous patterns in docker-compose/k8s
volumes:
  - /:/host              # entire host FS
  - /var/run/docker.sock # Docker socket = full host control
  - /proc:/host/proc     # host process list
  - /sys:/host/sys       # host kernel interface
```

### cgroup release_agent (CVE-2022-0492)
```bash
# Check if cgroup v1 writable
cat /proc/1/cgroup
# If vulnerable, can write to release_agent to run on host
```

### runc Overwrite (CVE-2019-5736)
- Overwrite `/proc/self/exe` → replaces runc binary on host
- Fixed in runc >= 1.0-rc7

## DuckOps Container Hardening

DuckOps scanner containers are hardened against all these:
```go
CapDrop:     strslice.StrSlice{"ALL"}  // drop everything
SecurityOpt: []string{"no-new-privileges:true"}
// No host path mounts (except :ro workspace)
// No docker socket mount
// PidsLimit: 100
// Memory: 512MB
```

## IaC Scanner Checks

Checkov, tfsec, and kics detect:
- `privileged: true` in pod specs
- `hostPID: true`, `hostNetwork: true`, `hostIPC: true`
- Dangerous volume mounts
- Missing `securityContext` blocks
- `allowPrivilegeEscalation: true`

## Kubernetes Specific

```yaml
# Hardened securityContext
securityContext:
  runAsNonRoot: true
  runAsUser: 10001
  allowPrivilegeEscalation: false
  readOnlyRootFilesystem: true
  capabilities:
    drop: ["ALL"]
  seccompProfile:
    type: RuntimeDefault
```
