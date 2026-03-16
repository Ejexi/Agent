//go:build !windows
package warden

import (
	"os/exec"
	"syscall"
)

func applySysProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true, // Create a new session to detach from calling terminal
	}
}
