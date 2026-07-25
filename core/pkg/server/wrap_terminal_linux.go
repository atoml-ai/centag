//go:build linux

package server

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func openSystemTerminal(commandLine string) error {
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
	if lastErr != nil {
		return fmt.Errorf("open terminal: %w", lastErr)
	}
	return fmt.Errorf("no terminal emulator found")
}
