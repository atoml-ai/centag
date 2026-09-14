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

// writeConfigOnLaunch is toggled by the tray; when on, the sidecar writes the
// app's local model config (centag/<default pipeline>) before launch. Default
// off: the transparent pipeline already maps the model.
var writeConfigOnLaunch bool

// runAgentByApp launches a catalog app through `centag wrap run`. Optionally
// asks the sidecar to write the app's model config first (best-effort; failures
// degrade to a warning and still launch under the transparent pipeline).
func runAgentByApp(a *launcherApp, app catalogApp) {
	// Desktop GUI apps use per-process proxy switches only; the OS system proxy
	// is never modified.
	if app.LaunchMode == launchModeSystemProxy {
		if err := launchSystemProxyApp(a, app); err != nil {
			msg := fmt.Sprintf("无法启动 %s: %v", app.DisplayName, err)
			fmt.Fprintf(os.Stderr, "centag-launcher: %s\n", msg)
			notifyUser("Centag", msg)
			return
		}
		notifyUser("Centag", "已通过进程级代理启动: "+app.DisplayName)
		return
	}

	notice := ""
	if writeConfigOnLaunch {
		if res, err := prepareWrapApp(a.cfg.baseURL(), app.ID, true); err != nil {
			notice = "写模型配置失败（继续透明启动）: " + err.Error()
		} else if w := warningsText(res); w != "" {
			notice = "模型配置提示: " + w
		}
	}

	argv := app.Argv
	if len(argv) == 0 {
		argv = []string{app.ID}
	}
	cmdLine := buildWrapRunArgvCommand(a, argv)

	// wrap_run targets are interactive CLI/TUI agents: they need a real TTY, so
	// launch them in a visible terminal instead of detached-with-log.
	if err := launchInTerminal(cmdLine, app.DisplayName); err != nil {
		msg := fmt.Sprintf("无法启动 %s: %v", app.DisplayName, err)
		if notice != "" {
			msg += "（" + notice + "）"
		}
		fmt.Fprintf(os.Stderr, "centag-launcher: %s\n", msg)
		notifyUser("Centag", msg)
		return
	}
	msg := "已在终端代理启动: " + app.DisplayName
	if notice != "" {
		msg += "（" + notice + "）"
	}
	notifyUser("Centag", msg)
}

// runDoctorNow fetches readiness checks from the sidecar and surfaces a concise
// summary via notification (full detail stays in stderr).
func runDoctorNow(a *launcherApp) {
	res, err := wrapDoctor(a.cfg.baseURL())
	if err != nil {
		msg := "代理诊断失败: " + err.Error()
		fmt.Fprintf(os.Stderr, "centag-launcher: %s\n", msg)
		notifyUser("Centag", msg)
		return
	}
	summary := formatDoctor(res)
	fmt.Fprintf(os.Stderr, "centag-launcher: doctor: %s\n", summary)
	title := "Centag 代理就绪"
	if ok, _ := res["ok"].(bool); !ok {
		title = "Centag 代理有问题"
	}
	notifyUser(title, summary)
}

// buildWrapRunCommand builds `<centag> wrap run --server URL [--token T] -- <exe>`.
func buildWrapRunCommand(a *launcherApp, exe string) string {
	return buildWrapRunArgvCommand(a, []string{exe})
}

// buildWrapRunArgvCommand builds a wrap-run command line for argv.
// No token is needed for the default loopback sidecar; CENTAG_WRAP_TOKEN is honored
// when the sidecar requires LAN MITM proxy auth.
func buildWrapRunArgvCommand(a *launcherApp, argv []string) string {
	parts := []string{shellQuote(a.hub.binary), "wrap", "run", "--server", shellQuote(a.cfg.baseURL())}
	if tok := strings.TrimSpace(os.Getenv("CENTAG_WRAP_TOKEN")); tok != "" {
		parts = append(parts, "--token", shellQuote(tok))
	}
	parts = append(parts, "--")
	for _, x := range argv {
		parts = append(parts, shellQuote(x))
	}
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
