//go:build !darwin && !windows && !linux

package server

import "fmt"

func openSystemTerminal(commandLine string) error {
	return fmt.Errorf("open terminal unsupported on this OS")
}
