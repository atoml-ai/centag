//go:build linux

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// selectExecutable shows a native Linux file picker (zenity, fallback kdialog)
// and returns the chosen absolute path (empty if cancelled).
func selectExecutable() (string, error) {
	if p, err := exec.LookPath("zenity"); err == nil {
		out, err := exec.Command(p, "--file-selection", "--title=选择要代理启动的程序").Output()
		if err != nil {
			return "", nil // cancelled
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
	if p, err := exec.LookPath("kdialog"); err == nil {
		out, err := exec.Command(p, "--getopenfilename", ".", "Executable ()|").Output()
		if err != nil {
			return "", nil
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
	return "", fmt.Errorf("no file picker available (install zenity or kdialog)")
}

// openTerminal writes a shell script and opens it in the first available terminal
// emulator, running commandLine with a final "press enter to close" prompt.
func openTerminal(commandLine string) error {
	commandLine = strings.TrimSpace(commandLine)
	if commandLine == "" {
		return fmt.Errorf("empty command")
	}
	dir := filepath.Join(os.TempDir(), "centag-wrap-run")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("temp dir: %w", err)
	}
	f, err := os.CreateTemp(dir, "run-*.sh")
	if err != nil {
		return fmt.Errorf("create script: %w", err)
	}
	path := f.Name()
	script := "#!/bin/bash\ncd \"$HOME\" || true\n" + commandLine + "\necho; read -r -p '(exit '$?') 按回车关闭' _\n"
	if _, err := f.WriteString(script); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return err
	}
	_ = f.Close()
	_ = os.Chmod(path, 0o700)

	candidates := []struct {
		bin  string
		args []string
	}{
		{"gnome-terminal", []string{"--", "bash", path}},
		{"kgx", []string{"--", "bash", path}},
		{"konsole", []string{"-e", "bash", path}},
		{"xfce4-terminal", []string{"-e", "bash " + path}},
		{"x-terminal-emulator", []string{"-e", "bash", path}},
		{"xterm", []string{"-e", "bash", path}},
	}
	var lastErr error
	for _, c := range candidates {
		p, err := exec.LookPath(c.bin)
		if err != nil {
			continue
		}
		cmd := exec.Command(p, c.args...)
		if err := cmd.Start(); err != nil {
			lastErr = err
			continue
		}
		return nil
	}
	_ = os.Remove(path)
	if lastErr != nil {
		return fmt.Errorf("open terminal: %w", lastErr)
	}
	return fmt.Errorf("no terminal emulator found")
}
