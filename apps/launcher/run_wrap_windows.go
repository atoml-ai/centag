//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// selectExecutable shows a native picker for installed applications. Primary:
// enumerate Start-Menu shortcuts (all users + current user), resolve each .lnk
// to its target exe, and let the user pick from the native Out-GridView list —
// the Windows counterpart of the macOS application browser. Fallback: a plain
// file dialog rooted at Program Files when the grid is unavailable (e.g.
// Server Core) or no shortcuts are found.
func selectExecutable() (string, error) {
	if exe, ok, err := selectExecutableFromStartMenu(); err != nil {
		fmt.Fprintf(os.Stderr, "centag-launcher: start menu picker failed: %v (falling back to file dialog)\n", err)
	} else if ok {
		return exe, nil
	}
	return selectExecutableViaFileDialog()
}

const startMenuPickerScript = `$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Windows.Forms | Out-Null
$sh = New-Object -ComObject WScript.Shell
$dirs = @(
  (Join-Path $env:ProgramData 'Microsoft\Windows\Start Menu\Programs'),
  (Join-Path $env:APPDATA 'Microsoft\Windows\Start Menu\Programs')
)
$seen = @{}
$apps = New-Object System.Collections.Generic.List[object]
foreach ($d in $dirs) {
  if (-not (Test-Path $d)) { continue }
  Get-ChildItem -Path $d -Filter *.lnk -Recurse -ErrorAction SilentlyContinue | ForEach-Object {
    $t = ''
    try { $t = $sh.CreateShortcut($_.FullName).TargetPath } catch { }
    if ($t -and (Test-Path $t) -and $t.ToLower().EndsWith('.exe') -and -not $seen.ContainsKey($t.ToLower())) {
      $seen[$t.ToLower()] = $true
      $apps.Add([PSCustomObject]@{ Name = $_.BaseName; Exe = $t })
    }
  }
}
if ($apps.Count -eq 0) { exit 2 }
$picked = $apps | Sort-Object Name | Out-GridView -Title '选择要通过 Centag 代理启动的应用程序' -OutputMode Single
if ($picked) { $picked.Exe }`

// selectExecutableFromStartMenu returns (exe, true, nil) when the user picks an
// app, ("", false, nil) when the list is empty, and an error when the grid UI
// is unavailable or the user cancelled via the grid close button.
func selectExecutableFromStartMenu() (string, bool, error) {
	out, err := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-STA", "-Command", startMenuPickerScript).Output()
	if err != nil {
		exitCode := -1
		if ee, ok := err.(*exec.ExitError); ok {
			exitCode = ee.ExitCode()
		}
		if exitCode == 2 {
			return "", false, nil // no shortcuts found → use the file dialog
		}
		return "", false, err
	}
	exe := strings.TrimSpace(string(out))
	if exe == "" {
		return "", false, nil // user dismissed the grid → treat as cancel
	}
	if _, err := os.Stat(exe); err != nil {
		return "", false, fmt.Errorf("selected application invalid: %w", err)
	}
	return exe, true, nil
}

// selectExecutableViaFileDialog is the fallback: a native file dialog rooted at
// Program Files, filtered to executables.
func selectExecutableViaFileDialog() (string, error) {
	script := `$ErrorActionPreference = 'Stop'
[void][System.Reflection.Assembly]::LoadWithPartialName("System.Windows.Forms")
$d = New-Object System.Windows.Forms.OpenFileDialog
$d.Title = "选择要通过 Centag 代理启动的程序"
$d.Filter = "应用程序 (*.exe;*.bat;*.cmd)|*.exe;*.bat;*.cmd|所有文件 (*.*)|*.*"
$d.CheckFileExists = $true
$d.CheckPathExists = $true
if ($d.ShowDialog() -eq [System.Windows.Forms.DialogResult]::OK) { $d.FileName }`
	out, err := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script).Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == -1073741510 {
			return "", nil // user cancelled (STATUS_CONTROL_C_EXIT variants aside)
		}
		return "", fmt.Errorf("open file dialog: %w", err)
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

// splitCommand splits a shell-style command line into argv, honoring double
// quotes (our shellQuote output). Single quotes are treated as literal.
func splitCommand(s string) []string {
	var args []string
	var cur strings.Builder
	inQuote := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '"':
			inQuote = !inQuote
		case c == ' ' && !inQuote:
			if cur.Len() > 0 {
				args = append(args, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteByte(c)
		}
	}
	if cur.Len() > 0 {
		args = append(args, cur.String())
	}
	return args
}

// launchWrapped starts the wrapped app in the background without any terminal
// window (CREATE_NO_WINDOW), mirroring the macOS behavior. Combined output is
// captured to a timestamped log under %USERPROFILE%\.centag\wrap\runs.
func launchWrapped(commandLine, label string) (string, error) {
	commandLine = strings.TrimSpace(commandLine)
	if commandLine == "" {
		return "", fmt.Errorf("empty command")
	}
	dir := filepath.Join(os.Getenv("USERPROFILE"), ".centag", "wrap", "runs")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("log dir: %w", err)
	}
	logPath := filepath.Join(dir, fmt.Sprintf("%s-%s.log", time.Now().Format("20060102-150405"), sanitizeLogLabel(label)))
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return "", fmt.Errorf("create log: %w", err)
	}
	args := splitCommand(commandLine)
	if len(args) == 0 {
		_ = f.Close()
		return "", fmt.Errorf("empty command")
	}
	// Exec argv directly: Go's Windows arg-quoting is always correct (never
	// route through cmd.exe, which re-parses quotes on zh-CN systems).
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdout = f
	cmd.Stderr = f
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: _CREATE_NO_WINDOW}
	if err := cmd.Start(); err != nil {
		_ = f.Close()
		return "", fmt.Errorf("start wrapped app: %w", err)
	}
	go func() {
		_ = cmd.Wait()
		_ = f.Close()
	}()
	return logPath, nil
}

// trustCACert installs the Centag CA into the Windows certificate stores
// (CurrentUser Root + CA) via certutil; no elevation required per-store.
func trustCACert(cfg Config) (string, error) {
	caPath := locateCentagCA(cfg)
	if caPath == "" {
		return "", fmt.Errorf("CA certificate not found (start the sidecar first)")
	}
	if out, err := exec.Command("certutil", "-user", "-addstore", "Root", caPath).CombinedOutput(); err != nil {
		return "", fmt.Errorf("certutil Root: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	_ = exec.Command("certutil", "-user", "-addstore", "CA", caPath).Run()
	return caPath, nil
}

// locateCentagCA finds the sidecar's root CA on disk.
func locateCentagCA(cfg Config) string {
	for _, p := range []string{
		filepath.Join(cfg.DataDir, "certs", "ca.crt"),
		filepath.Join(os.Getenv("USERPROFILE"), ".centag", "wrap", "ca.crt"),
	} {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}

// ensureCATrusted silently installs the CA into the current-user certificate
// stores on every launch (idempotent; no admin required on Windows).
func ensureCATrusted(cfg Config) {
	if cfg.Headless {
		return
	}
	caPath := locateCentagCA(cfg)
	if caPath == "" {
		return
	}
	_ = exec.Command("certutil", "-f", "-user", "-addstore", "Root", caPath).Run()
	_ = exec.Command("certutil", "-f", "-user", "-addstore", "CA", caPath).Run()
}
