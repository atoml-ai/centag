package server

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode"
)

// wrapPreset is a CLI agent that can be launched via `centag wrap run`.
type wrapPreset struct {
	ID          string   `json:"id"`
	DisplayName string   `json:"display_name"`
	Description string   `json:"description"`
	Argv        []string `json:"argv"`
}

// wrapPresets lists supported wrap-launch targets (CLI/TUI only; no desktop GUI apps).
func wrapPresets() []wrapPreset {
	return []wrapPreset{
		{ID: "opencode", DisplayName: "OpenCode", Description: "opencode.ai CLI", Argv: []string{"opencode"}},
		{ID: "claude-code", DisplayName: "Claude Code", Description: "Anthropic Claude Code CLI", Argv: []string{"claude"}},
		{ID: "codex", DisplayName: "Codex", Description: "OpenAI Codex CLI", Argv: []string{"codex"}},
		{ID: "gemini-cli", DisplayName: "Gemini CLI", Description: "Google Gemini CLI", Argv: []string{"gemini"}},
		{ID: "grok-build", DisplayName: "Grok", Description: "xAI Grok CLI", Argv: []string{"grok"}},
		{ID: "hermes", DisplayName: "Hermes", Description: "Hermes Agent CLI", Argv: []string{"hermes"}},
		{ID: "openclaw", DisplayName: "OpenClaw", Description: "OpenClaw CLI", Argv: []string{"openclaw"}},
		{ID: "codebuddy", DisplayName: "CodeBuddy", Description: "Tencent CodeBuddy CLI", Argv: []string{"codebuddy"}},
	}
}

// parseWrapArgv resolves argv from request body fields.
// Prefer Argv when non-empty; otherwise split Command on whitespace.
// Rejects shell metacharacters so the string is never interpreted by a shell.
func parseWrapArgv(argv []string, command string) ([]string, error) {
	if len(argv) > 0 {
		for _, a := range argv {
			if err := rejectShellMeta(a); err != nil {
				return nil, err
			}
			if strings.TrimSpace(a) == "" {
				return nil, fmt.Errorf("argv contains empty element")
			}
		}
		return argv, nil
	}
	command = strings.TrimSpace(command)
	if command == "" {
		return nil, fmt.Errorf("argv or command is required")
	}
	if err := rejectShellMeta(command); err != nil {
		return nil, err
	}
	parts := splitCommandArgs(command)
	if len(parts) == 0 {
		return nil, fmt.Errorf("command is empty")
	}
	return parts, nil
}

func rejectShellMeta(s string) error {
	if strings.ContainsAny(s, ";|&$`<>(){}[]!\\\"'\n\r") {
		return fmt.Errorf("command contains disallowed characters")
	}
	return nil
}

// splitCommandArgs splits on unicode whitespace (no shell quoting support).
func splitCommandArgs(command string) []string {
	fields := strings.FieldsFunc(command, unicode.IsSpace)
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if f != "" {
			out = append(out, f)
		}
	}
	return out
}

func wrapPresetByID(id string) (wrapPreset, bool) {
	id = strings.TrimSpace(id)
	for _, p := range wrapPresets() {
		if p.ID == id {
			return p, true
		}
	}
	return wrapPreset{}, false
}

func isLoopbackIP(host string) bool {
	host = strings.TrimSpace(host)
	if host == "" {
		return false
	}
	// Gin ClientIP may return "127.0.0.1" or IPv6.
	if host == "localhost" || host == "::1" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// buildWrapRunCommandLine builds a shell-quoted command using an absolute exe path
// (for Terminal scripts). Prefer resolveWrapExecutable() for the exe argument.
func buildWrapRunCommandLine(exe, server, token string, argv []string) (string, error) {
	if strings.TrimSpace(exe) == "" {
		return "", fmt.Errorf("executable path is empty")
	}
	if len(argv) == 0 {
		return "", fmt.Errorf("argv is empty")
	}
	parts := []string{shellQuote(exe), "wrap", "run"}
	if s := strings.TrimSpace(server); s != "" {
		parts = append(parts, "--server", shellQuote(s))
	}
	if t := strings.TrimSpace(token); t != "" {
		parts = append(parts, "--token", shellQuote(t))
	}
	parts = append(parts, "--")
	for _, a := range argv {
		parts = append(parts, shellQuote(a))
	}
	return strings.Join(parts, " "), nil
}

// buildWrapRunUserCommand is the short form shown/copied in the UI:
//
//	centag wrap run [--server URL] [--token KEY] -- <argv...>
func buildWrapRunUserCommand(server, token string, argv []string) (string, error) {
	if len(argv) == 0 {
		return "", fmt.Errorf("argv is empty")
	}
	parts := []string{"centag", "wrap", "run"}
	if s := strings.TrimSpace(server); s != "" {
		parts = append(parts, "--server", s)
	}
	if t := strings.TrimSpace(token); t != "" {
		parts = append(parts, "--token", t)
	}
	parts = append(parts, "--")
	parts = append(parts, argv...)
	return strings.Join(parts, " "), nil
}

// resolveWrapExecutable picks a binary that can run `wrap` subcommand.
func resolveWrapExecutable() (string, error) {
	home, _ := os.UserHomeDir()
	candidates := []string{}
	if home != "" {
		candidates = append(candidates,
			filepath.Join(home, ".centag", "bin", "centag"),
			filepath.Join(home, ".centag", "bin", "centag-personal"),
		)
	}
	if exe, err := os.Executable(); err == nil && exe != "" {
		if resolved, err := filepath.EvalSymlinks(exe); err == nil {
			candidates = append(candidates, resolved)
		} else {
			candidates = append(candidates, exe)
		}
	}
	if p, err := exec.LookPath("centag"); err == nil {
		candidates = append(candidates, p)
	}
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return c, nil
		}
	}
	return "", fmt.Errorf("centag executable not found (install CLI or run desktop sidecar)")
}

func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	// Prefer single quotes; escape embedded single quotes the POSIX way: 'foo'\''bar'
	if !strings.ContainsAny(s, " \t\n'\"\\$`;&|<>(){}[]!*?") && s != "" {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func resolveSelfExecutable() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return exe, nil
}
