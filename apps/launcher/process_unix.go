//go:build unix

package main

import (
	"os"
	"syscall"
)

func stopProcess(p *os.Process) {
	_ = p.Signal(syscall.SIGTERM)
}
