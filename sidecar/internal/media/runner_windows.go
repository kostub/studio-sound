//go:build windows

package media

import (
	"fmt"
	"os/exec"
	"strings"

	"golang.org/x/sys/windows"
)

func setProcAttrs(cmd *exec.Cmd) {
	cmd.SysProcAttr = &windows.SysProcAttr{CreationFlags: windows.CREATE_NEW_PROCESS_GROUP}
}

func killGroup(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	kill := exec.Command("taskkill", "/T", "/F", "/PID", fmt.Sprint(cmd.Process.Pid))
	return kill.Run()
}

func maybeLongPath(p string) string {
	if len(p) <= 240 {
		return p
	}
	if strings.HasPrefix(p, `\\?\`) {
		return p
	}
	return `\\?\` + p
}
