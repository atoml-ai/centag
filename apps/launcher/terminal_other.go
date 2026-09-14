//go:build !windows && !darwin && !linux

package main

import "fmt"

// launchInTerminal is unsupported on this OS.
func launchInTerminal(commandLine, label string) error {
	_ = commandLine
	_ = label
	return fmt.Errorf("open terminal unsupported on this OS")
}
