//go:build windows

package main

import "os"

func stopProcess(p *os.Process) {
	_ = p.Kill()
}
