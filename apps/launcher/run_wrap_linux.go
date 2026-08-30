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

// launchWrapped starts the wrapped app in a terminal emulator (Linux keeps a
// visible terminal; see openTerminal).
func launchWrapped(commandLine, label string) (string, error) {
	return "", openTerminal(commandLine)
}

// trustCACert installs the Centag CA into the user NSS/CA store via
// update-ca-trust or certutil when available.
func trustCACert(cfg Config) (string, error) {
	caPath := locateCentagCA(cfg)
	if caPath == "" {
		return "", fmt.Errorf("CA certificate not found (start the sidecar first)")
	}
	if p, err := exec.LookPath("update-ca-trust"); err == nil {
		dst := "/etc/pki/ca-trust/source/anchors/centag-ca.crt"
		if out, err := exec.Command("pkexec", "cp", caPath, dst).CombinedOutput(); err != nil {
			return "", fmt.Errorf("pkexec cp: %w (%s)", err, strings.TrimSpace(string(out)))
		}
		if out, err := exec.Command(p, "extract").CombinedOutput(); err != nil {
			return "", fmt.Errorf("update-ca-trust: %w (%s)", err, strings.TrimSpace(string(out)))
		}
		return caPath, nil
	}
	return "", fmt.Errorf("请手动将 %s 加入系统信任（未找到 update-ca-trust）", caPath)
}

// locateCentagCA finds the sidecar's root CA on disk.
func locateCentagCA(cfg Config) string {
	home, _ := os.UserHomeDir()
	for _, p := range []string{
		filepath.Join(cfg.DataDir, "certs", "ca.crt"),
		filepath.Join(home, ".centag", "wrap", "ca.crt"),
	} {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}

// ensureCATrusted silently adds the CA to the user NSS database read by
// Chrome/Chromium; system-wide trust needs root and is left to the tray item.
func ensureCATrusted(cfg Config) {
	if cfg.Headless {
		return
	}
	caPath := locateCentagCA(cfg)
	if caPath == "" {
		return
	}
	nss, err := exec.LookPath("certutil")
	if err != nil {
		return // NSS tools not installed; nothing silent we can do
	}
	home, _ := os.UserHomeDir()
	db := filepath.Join(home, ".pki", "nssdb")
	_ = os.MkdirAll(db, 0o700)
	_ = exec.Command(nss, "-d", "sql:"+db, "-A", "-t", "C,,", "-n", "Centag CA", "-i", caPath).Run()
}
