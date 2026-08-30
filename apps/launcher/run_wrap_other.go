//go:build !windows && !darwin && !linux

package main

import "fmt"

func selectExecutable() (string, error) {
	return "", fmt.Errorf("file picker unsupported on this OS")
}

func openTerminal(commandLine string) error {
	return fmt.Errorf("open terminal unsupported on this OS")
}

// launchWrapped is unsupported on this OS.
func launchWrapped(commandLine, label string) (string, error) {
	return "", openTerminal(commandLine)
}

// trustCACert is unsupported on this OS.
func trustCACert(cfg Config) (string, error) {
	return "", fmt.Errorf("trust CA unsupported on this OS")
}

// locateCentagCA finds the sidecar's root CA on disk.
func locateCentagCA(cfg Config) string {
	return filepath.Join(cfg.DataDir, "certs", "ca.crt")
}

// ensureCATrusted is a no-op on this OS.
func ensureCATrusted(cfg Config) {}
