//go:build !windows

package main

import (
	"os/exec"
	"syscall"
)

func applyBackgroundProcessAttrs(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
