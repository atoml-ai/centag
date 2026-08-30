package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// runAgentViaWrap opens a native dialog to pick a local executable, then launches
// it through `centag wrap run` so its LLM traffic is proxied by the sidecar.
// Replaces the old "open /agent-run page" tray behavior.
func runAgentViaWrap(a *launcherApp) {
	exe, err := selectExecutable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "centag-launcher: select executable: %v\n", err)
		notifyUser("Centag", "选择程序失败: "+err.Error())
		return
	}
	if strings.TrimSpace(exe) == "" {
		return // user cancelled
	}

	cmdLine := buildWrapRunCommand(a, exe)
	extra, err := launchWrapped(cmdLine, filepath.Base(exe))
	if err != nil {
		msg := fmt.Sprintf("无法启动 %s: %v", exe, err)
		fmt.Fprintf(os.Stderr, "centag-launcher: %s\n", msg)
		notifyUser("Centag", msg)
		return
	}
	msg := "已通过代理启动: " + filepath.Base(exe)
	if extra != "" {
		msg += "（日志: " + extra + "）"
	}
	notifyUser("Centag", msg)
}

// buildWrapRunCommand builds `<centag> wrap run --server URL [--token T] -- <exe>`.
// No token is needed for the default loopback sidecar; CENTAG_WRAP_TOKEN is honored
// when the sidecar requires LAN MITM proxy auth.
func buildWrapRunCommand(a *launcherApp, exe string) string {
	parts := []string{shellQuote(a.hub.binary), "wrap", "run", "--server", shellQuote(a.cfg.baseURL())}
	if tok := strings.TrimSpace(os.Getenv("CENTAG_WRAP_TOKEN")); tok != "" {
		parts = append(parts, "--token", shellQuote(tok))
	}
	parts = append(parts, "--", shellQuote(exe))
	return strings.Join(parts, " ")
}

// sanitizeLogLabel keeps only filename-safe characters for log file names.
func sanitizeLogLabel(label string) string {
	var b strings.Builder
	for _, r := range label {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9',
			r == '.', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	if b.Len() == 0 {
		return "app"
	}
	return b.String()
}

// shellQuote wraps s for safe embedding in the OS shell command line.
// On Windows (cmd.exe) double quotes are used; elsewhere single quotes (sh/bash).
func shellQuote(s string) string {
	if runtime.GOOS == "windows" {
		if s == "" {
			return "\"\""
		}
		// cmd.exe treats a leading/trailing backslash before a closing quote
		// specially, so escape embedded double quotes by doubling them.
		return "\"" + strings.ReplaceAll(s, "\"", "\"\"") + "\""
	}
	if s == "" {
		return "''"
	}
	if !strings.ContainsAny(s, " \t\n'\"\\$`;&|<>(){}[]!*?") {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
