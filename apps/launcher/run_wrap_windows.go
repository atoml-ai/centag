//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

// selectExecutable shows a native Windows "Open File" dialog filtered to
// executables, returning the chosen absolute path (empty if cancelled).
func selectExecutable() (string, error) {
	script := `[void][System.Reflection.Assembly]::LoadWithPartialName("System.Windows.Forms")
$d = New-Object System.Windows.Forms.OpenFileDialog
$d.Title = "选择要代理启动的程序"
$d.Filter = "可执行文件 (*.exe)|*.exe|所有文件 (*.*)|*.*"
$d.CheckFileExists = $true
$d.CheckPathExists = $true
if ($d.ShowDialog() -eq [System.Windows.Forms.DialogResult]::OK) { $d.FileName }`
	out, err := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script).Output()
	if err != nil {
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

// openTerminal launches commandLine in a new, visible console window.
//
// We deliberately do NOT route the line through `cmd.exe /k`, because cmd.exe
// re-parses the string and any quoting nuance (single vs double quotes, paths
// with spaces/backslashes) produces "文件名、目录名或卷标语法不正确" on zh-CN
// Windows. Instead we split the line into argv and exec the binary directly;
// Go's Windows arg-quoting is native and always correct. CREATE_NEW_CONSOLE
// gives the child its own console window so output is visible.
func openTerminal(commandLine string) error {
	commandLine = strings.TrimSpace(commandLine)
	if commandLine == "" {
		return fmt.Errorf("empty command")
	}
	args := splitCommand(commandLine)
	if len(args) == 0 {
		return fmt.Errorf("empty command")
	}
	cmd := exec.Command(args[0], args[1:]...)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x00000010} // CREATE_NEW_CONSOLE
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("open terminal: %w", err)
	}
	return nil
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
