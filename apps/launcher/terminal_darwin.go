//go:build darwin

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// launchInTerminal writes a double-clickable .command script and opens it in
// Terminal.app so interactive TUI agents get a real TTY. Mirrors
// core/pkg/server openSystemTerminal.
func launchInTerminal(commandLine, _ string) error {
	commandLine = strings.TrimSpace(commandLine)
	if commandLine == "" {
		return fmt.Errorf("empty command")
	}
	dir := filepath.Join(os.TempDir(), "centag-wrap-run")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("temp dir: %w", err)
	}
	f, err := os.CreateTemp(dir, "run-*.command")
	if err != nil {
		return fmt.Errorf("create .command: %w", err)
	}
	path := f.Name()
	script := "#!/bin/bash\n" +
		"cd \"$HOME\" || true\n" +
		"clear\n" +
		commandLine + "\n" +
		"status=$?\n" +
		"echo\n" +
		"echo \"(exit $status) 按回车关闭\"\n" +
		"read -r _\n"
	if _, err := f.WriteString(script); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return fmt.Errorf("write .command: %w", err)
	}
	_ = f.Close()
	if err := os.Chmod(path, 0o700); err != nil {
		_ = os.Remove(path)
		return err
	}
	if out, err := exec.Command("open", path).CombinedOutput(); err != nil {
		return fmt.Errorf("open %s: %w (%s)", path, err, strings.TrimSpace(string(out)))
	}
	return nil
}
