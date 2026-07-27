//go:build !windows

package main

import (
	"os"
	"os/exec"
)

func stopProcess(p *os.Process) {
	_ = p.Kill()
}

func hideSidecarWindow(cmd *exec.Cmd) {}
