package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// installCentagCLI exposes the installed sidecar binary as `centag` on the
// user's PATH so `centag wrap …` works from a terminal. Entry layout matches
// scripts/install.sh:
//
//  1. <root>/bin/centag        — symlink/shim to lib/<edition>/centag-<edition> (no admin)
//  2. <root>/env               — PATH helper written when missing (`source ~/.centag/env`)
//  3. PATH fallback per OS:
//     darwin  → /usr/local/bin/centag          (admin prompt via osascript)
//     linux   → ~/.local/bin/centag            (no privilege escalation)
//     windows → %LOCALAPPDATA%\Microsoft\WindowsApps\centag.cmd
func installCentagCLI(binaryPath string) error {
	if binaryPath == "" {
		return fmt.Errorf("sidecar binary not resolved")
	}
	if _, err := os.Stat(binaryPath); err != nil {
		return fmt.Errorf("sidecar binary %s: %w", binaryPath, err)
	}

	rootBinDir := filepath.Join(centagInstallRoot(), "bin")
	if err := os.MkdirAll(rootBinDir, 0o755); err != nil {
		return err
	}
	entry := filepath.Join(rootBinDir, "centag.cmd")
	if runtime.GOOS != "windows" {
		entry = filepath.Join(rootBinDir, "centag")
		_ = os.Remove(entry)
		if err := os.Symlink(binaryPath, entry); err != nil {
			return err
		}
	} else {
		shim := fmt.Sprintf("@\"%s\" %%*\r\n", binaryPath)
		if err := os.WriteFile(entry, []byte(shim), 0o755); err != nil {
			return err
		}
	}
	if err := writeEnvFileIfMissing(centagInstallRoot()); err != nil {
		fmt.Fprintf(os.Stderr, "centag-launcher: warn: write env helper: %v\n", err)
	}

	fmt.Fprintf(os.Stderr, "centag-launcher: `centag` installed at %s -> %s\n", entry, binaryPath)

	// On macOS the authoritative reachability marker is the /usr/local/bin
	// symlink (GUI process PATH is unreliable for this check): once it points
	// at the current binary, every new terminal can run `centag` and re-running
	// the menu item stays silent (no extra authorization prompts).
	if runtime.GOOS == "darwin" && cliDarwinInstalled(binaryPath) {
		return nil
	}

	if pathContainsDir(rootBinDir) {
		return nil
	}

	// <root>/bin not on PATH — install a per-OS fallback entry.
	var err error
	switch runtime.GOOS {
	case "darwin":
		err = installCLIDarwinFallback(binaryPath)
	case "linux":
		err = installCLILinuxFallback(binaryPath)
	case "windows":
		err = installCLIWindowsFallback(binaryPath)
	default:
		return nil
	}
	if err != nil {
		return fmt.Errorf("入口已写入 %s，但 %s 不在 PATH：请 `source %s` 或将其加入 PATH（%v）",
			entry, rootBinDir, filepath.Join(centagInstallRoot(), "env"), err)
	}
	return nil
}

// installCLIDarwinFallback makes `centag` reachable from every NEW terminal
// with a SINGLE admin authorization:
//   - symlink /usr/local/bin/centag → sidecar binary (on macOS default PATH)
//   - register <root>/bin in /etc/paths.d/centag so future centag tools are
//     also resolvable
//
// Re-running is silent when the symlink already points at the current binary
// (no repeated authorization prompts).
func installCLIDarwinFallback(binaryPath string) error {
	const target = "/usr/local/bin/centag"
	if cliDarwinInstalled(binaryPath) {
		return nil
	}
	script := fmt.Sprintf(
		`do shell script "mkdir -p /usr/local/bin && ln -sf '%s' '%s' && echo '%s' > /etc/paths.d/centag" with administrator privileges`,
		shellQuoteSingle(binaryPath), target, shellQuoteSingle(centagInstallRoot()+"/bin"))
	out, err := runCommand(30*time.Second, "osascript", "-e", script)
	if err != nil {
		msg := strings.TrimSpace(out)
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("需要管理员授权: %s", msg)
	}
	return nil
}

// cliDarwinInstalled reports whether /usr/local/bin/centag already resolves to
// the current sidecar binary. Both sides are symlink-normalized before
// comparing (macOS resolves /var/... to /private/var/...).
func cliDarwinInstalled(binaryPath string) bool {
	const target = "/usr/local/bin/centag"
	st, err := os.Lstat(target)
	if err != nil || st.Mode()&os.ModeSymlink == 0 {
		return false
	}
	resolved, err := filepath.EvalSymlinks(target)
	if err != nil {
		return false
	}
	want, err := filepath.EvalSymlinks(binaryPath)
	if err != nil {
		want = binaryPath
	}
	return resolved == want
}

func installCLILinuxFallback(binaryPath string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	binDir := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return err
	}
	target := filepath.Join(binDir, "centag")
	_ = os.Remove(target)
	return os.Symlink(binaryPath, target)
}

func installCLIWindowsFallback(binaryPath string) error {
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		localAppData = filepath.Join(home, "AppData", "Local")
	}
	binDir := filepath.Join(localAppData, "Microsoft", "WindowsApps")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return err
	}
	shim := fmt.Sprintf("@\"%s\" %%*\r\n", binaryPath)
	return os.WriteFile(filepath.Join(binDir, "centag.cmd"), []byte(shim), 0o755)
}

func pathContainsDir(dir string) bool {
	for _, p := range filepath.SplitList(os.Getenv("PATH")) {
		if p == dir {
			return true
		}
	}
	return false
}

// writeEnvFileIfMissing creates the PATH helper at <root>/env when absent
// (scripts/install.sh owns the file when it already exists).
func writeEnvFileIfMissing(root string) error {
	envPath := filepath.Join(root, "env")
	if fileExists(envPath) {
		return nil
	}
	content := "# Centag CLI PATH helper — source this file in your shell profile:\n" +
		"#   . \"" + root + "/env\"\n" +
		"export PATH=\"" + root + "/bin:$PATH\"\n"
	return os.WriteFile(envPath, []byte(content), 0o644)
}

// notifyUser shows a best-effort system notification (macOS notification /
// Windows balloon tip rendered as a toast on Win10+).
func notifyUser(title, message string) {
	switch runtime.GOOS {
	case "darwin":
		esc := func(s string) string {
			return strings.ReplaceAll(strings.ReplaceAll(s, `\`, `\\`), `"`, `\"`)
		}
		script := fmt.Sprintf(`display notification "%s" with title "%s"`, esc(message), esc(title))
		_, _ = runCommand(5*time.Second, "osascript", "-e", script)
	case "windows":
		psq := func(s string) string { return strings.ReplaceAll(s, "'", "''") }
		script := fmt.Sprintf(`
Add-Type -AssemblyName System.Windows.Forms | Out-Null
Add-Type -AssemblyName System.Drawing | Out-Null
$n = New-Object System.Windows.Forms.NotifyIcon
$n.Icon = [System.Drawing.SystemIcons]::Information
$n.Visible = $true
$n.BalloonTipTitle = '%s'
$n.BalloonTipText = '%s'
$n.ShowBalloonTip(5000)
Start-Sleep -Seconds 6
$n.Dispose()`, psq(title), psq(message))
		_, _ = runCommand(15*time.Second, "powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	}
}

// shellQuoteSingle escapes a path for embedding inside single quotes in shell.
func shellQuoteSingle(s string) string {
	return strings.ReplaceAll(s, "'", `'\''`)
}
