//go:build windows

package platform

import (
	"os/exec"
	"syscall"
)

const createNoWindow = 0x08000000

func HideCommandWindow(cmd *exec.Cmd) *exec.Cmd {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}
	return cmd
}
