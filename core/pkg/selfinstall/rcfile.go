package selfinstall

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// rcfile.go holds POSIX rc-file text handling shared by the unix and windows
// PATH persistence implementations. On Windows both the registry PATH and
// the bash rc file are modified so that CMD/PowerShell AND Git Bash/MSYS2
// pick up the new PATH.

func homeDir() string {
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Clean(home)
	}
	return "."
}

func xdgConfigHome() string {
	if xdg := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); xdg != "" {
		return xdg
	}
	return filepath.Join(homeDir(), ".config")
}

func zdotDir() string {
	if zdot := strings.TrimSpace(os.Getenv("ZDOTDIR")); zdot != "" {
		return zdot
	}
	return homeDir()
}

// pickRcFile selects the first existing rc file, falling back to creating the
// shell's primary one (mirrors install.sh ensure_path).
func pickRcFile(candidates []string) (string, error) {
	primary := candidates[0]
	for _, f := range candidates {
		if fi, err := os.Stat(f); err == nil && !fi.IsDir() {
			return f, nil
		}
	}
	if err := os.MkdirAll(filepath.Dir(primary), 0o755); err != nil {
		return "", err
	}
	// Creating the primary rc keeps PATH persistent for new terminals.
	if err := os.WriteFile(primary, nil, 0o644); err != nil {
		return "", fmt.Errorf("cannot create %s (add the PATH entry manually): %w", primary, err)
	}
	return primary, nil
}

// currentShell mirrors install.sh: basename of $SHELL, default bash.
// On Windows this returns "bash" when running inside Git Bash/MSYS2.
func currentShell() string {
	shell := strings.TrimSpace(os.Getenv("SHELL"))
	if shell == "" {
		return "bash"
	}
	return strings.TrimSuffix(filepath.Base(shell), ".exe")
}

// rcCandidates lists rc files in install.sh precedence order for the shell.
func rcCandidates(shell string, home, xdg, zdot string) []string {
	switch shell {
	case "fish":
		return []string{filepath.Join(home, ".config", "fish", "config.fish")}
	case "zsh":
		return []string{
			filepath.Join(zdot, ".zshrc"),
			filepath.Join(zdot, ".zshenv"),
			filepath.Join(xdg, "zsh", ".zshrc"),
		}
	case "bash":
		return []string{
			filepath.Join(home, ".bashrc"),
			filepath.Join(home, ".bash_profile"),
			filepath.Join(home, ".profile"),
		}
	default:
		return []string{
			filepath.Join(home, ".bashrc"),
			filepath.Join(home, ".profile"),
		}
	}
}

// rcSourceLine is the rc line that activates the env helper.
func rcSourceLine(root string) string {
	if currentShell() == "fish" {
		return fmt.Sprintf("source \"%s/env.fish\"", toSlash(root))
	}
	return fmt.Sprintf("[ -f \"%s/env\" ] && . \"%s/env\"", toSlash(root), toSlash(root))
}

// rcReferencesEntry reports whether rc content already activates the env
// helper or inlines the bin dir (dedup guard, covers install.sh output and
// legacy inline exports).
func rcReferencesEntry(content, root, binDir string) bool {
	return strings.Contains(content, toSlash(root)+"/env") ||
		strings.Contains(content, toSlash(binDir)) ||
		strings.Contains(content, binDir)
}

// filterRcLines removes centag PATH blocks: marker lines (install.sh or
// centag install) with their following activation lines, plus legacy inline
// exports. Returns the filtered lines and how many lines were removed.
func filterRcLines(lines []string, root, binDir string) ([]string, int) {
	out := make([]string, 0, len(lines))
	removed := 0
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		switch {
		case strings.Contains(line, "# centag (added by "):
			removed++
			// Drop the immediately following activation lines of the block.
			for j := i + 1; j < len(lines) && j <= i+3; j++ {
				if lines[j] == "" || rcReferencesEntry(lines[j], root, binDir) {
					removed++
					i = j
					continue
				}
				break
			}
		case isLegacyPathLine(line, root, binDir):
			removed++
		default:
			out = append(out, line)
		}
	}
	return out, removed
}

// AppendBashRcPath appends a source block to the shell rc file so that
// Git Bash / MSYS2 / WSL pick up the centag PATH on startup.  This is
// called from path_windows.go in addition to the registry write, and from
// path_unix.go as the sole persistence mechanism.
func AppendBashRcPath(root, binDir string) (pathResult, error) {
	rcFile, err := pickRcFile(rcCandidates(currentShell(), homeDir(), xdgConfigHome(), zdotDir()))
	if err != nil {
		return pathResult{}, err
	}
	existing, err := os.ReadFile(rcFile)
	if err != nil && !os.IsNotExist(err) {
		return pathResult{}, err
	}
	if rcReferencesEntry(string(existing), root, binDir) {
		return pathResult{detail: rcFile}, nil
	}
	block := fmt.Sprintf("\n# centag (added by centag install)\n%s\n", rcSourceLine(root))
	fh, err := os.OpenFile(rcFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return pathResult{}, fmt.Errorf("cannot write %s (add the PATH entry manually): %w", rcFile, err)
	}
	defer fh.Close()
	if _, err := fh.WriteString(block); err != nil {
		return pathResult{}, err
	}
	return pathResult{changed: true, detail: rcFile}, nil
}

// RemoveBashRcPath strips centag-added PATH blocks and legacy inline entries
// from all known rc files.  Called from both path_windows.go and path_unix.go.
func RemoveBashRcPath(root, binDir string) (pathResult, error) {
	home, xdg, zdot := homeDir(), xdgConfigHome(), zdotDir()
	var files []string
	seen := map[string]bool{}
	for _, shell := range []string{"bash", "zsh", "fish"} {
		for _, f := range rcCandidates(shell, home, xdg, zdot) {
			if !seen[f] {
				seen[f] = true
				files = append(files, f)
			}
		}
	}
	total := 0
	var touched []string
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		out, removed := filterRcLines(strings.Split(string(data), "\n"), root, binDir)
		if removed == 0 {
			continue
		}
		if err := os.WriteFile(f, []byte(strings.Join(out, "\n")), 0o644); err != nil {
			return pathResult{}, fmt.Errorf("rewrite %s: %w", f, err)
		}
		total += removed
		touched = append(touched, f)
	}
	detail := "no rc file referenced the entry"
	if total > 0 {
		detail = fmt.Sprintf("%d line(s) from %s", total, strings.Join(touched, ", "))
	}
	return pathResult{changed: total > 0, detail: detail}, nil
}

// isLegacyPathLine matches hand/install.sh-written inline PATH entries for
// the bin dir outside a marked block.
func isLegacyPathLine(line, root, binDir string) bool {
	if strings.Contains(line, toSlash(root)+"/env") {
		return true
	}
	if !strings.Contains(line, binDir) && !strings.Contains(line, toSlash(binDir)) {
		return false
	}
	return strings.Contains(line, "export PATH") || strings.Contains(line, "fish_add_path")
}
