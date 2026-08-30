package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// runAgentViaWrap opens a native dialog to pick a local executable, then launches
// it through `centag wrap run` in a system terminal so its LLM traffic is proxied
// by the sidecar. Replaces the old "open /agent-run page" tray behavior.
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
	if err := openTerminal(cmdLine); err != nil {
		msg := fmt.Sprintf("无法启动终端运行 %s: %v", exe, err)
		fmt.Fprintf(os.Stderr, "centag-launcher: %s\n", msg)
		notifyUser("Centag", msg)
		return
	}
	notifyUser("Centag", "已通过代理启动: "+filepath.Base(exe))
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
