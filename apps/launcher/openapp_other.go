//go:build !windows && !darwin && !linux

package main

import "fmt"

// openApp is unsupported on this OS.
func openApp(path, name string) error {
	_ = path
	_ = name
	return fmt.Errorf("open app unsupported on this OS")
}
