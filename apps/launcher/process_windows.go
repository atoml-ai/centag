//go:build windows

package main

import (
	"os"
	"os/exec"
	"syscall"
)

const _CREATE_NO_WINDOW = 0x08000000

func stopProcess(p *os.Process) {
	_ = p.Kill()
}

func hideSidecarWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: _CREATE_NO_WINDOW}
}
