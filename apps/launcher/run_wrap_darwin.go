//go:build darwin

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// selectExecutable shows a native macOS "choose file" dialog and returns the
// chosen POSIX path (empty if cancelled).
func selectExecutable() (string, error) {
	script := `POSIX path of (choose file with prompt "选择要代理启动的程序")`
	out, err := exec.Command("osascript", "-e", script).Output()
	if err != nil {
		// exit code 128 == user cancelled the dialog
		if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 128 {
			return "", nil
		}
		return "", fmt.Errorf("choose file: %w", err)
	}
	path := strings.TrimSpace(string(out))
	if path == "" {
		return "", nil
	}
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("selected path invalid: %w", err)
	}
	return path, nil
}

// openTerminal writes a .command script that runs commandLine and opens it in
// Terminal.app (more reliable than AppleScript "do script").
func openTerminal(commandLine string) error {
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
