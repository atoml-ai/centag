//go:build darwin

package server

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// openSystemTerminal writes a double-clickable .command script and opens it.
// More reliable than AppleScript "do script" (macOS Automation permissions).
func openSystemTerminal(commandLine string) error {
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
		"printf '%s\\n' " + shellQuote(commandLine) + "\n" +
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
		return fmt.Errorf("chmod .command: %w", err)
	}
	cmd := exec.Command("open", path)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("open %s: %w (%s)", path, err, strings.TrimSpace(string(out)))
	}
	return nil
}
