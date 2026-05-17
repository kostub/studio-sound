//go:build !windows

package media

import (
	"os/exec"
	"syscall"
)

func setProcAttrs(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func killGroup(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	// Negative pid kills the whole process group.
	return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
}

func maybeLongPath(p string) string { return p }
