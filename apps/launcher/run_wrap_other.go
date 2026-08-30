//go:build !windows && !darwin && !linux

package main

import "fmt"

func selectExecutable() (string, error) {
	return "", fmt.Errorf("file picker unsupported on this OS")
}

func openTerminal(commandLine string) error {
	return fmt.Errorf("open terminal unsupported on this OS")
}
