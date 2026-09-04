package selfinstall

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// shimPath returns bin/centag.cmd (windows) or bin/centag (unix).
func shimPath(L layout) string {
	if isWindowsRuntime {
		return filepath.Join(L.binDir, "centag.cmd")
	}
	return filepath.Join(L.binDir, "centag")
}

// writeShim writes the PATH entry shim. The wrapper contract is byte-for-byte
// the one from scripts/install.sh write_wrapper_centag (CLI mode, no desktop
// tray branch) so install.sh, centag-layout.sh and `centag install` stay
// interchangeable.
//
// On Windows both a .cmd shim (for CMD / PowerShell) and a bash shim (for
// Git Bash / MSYS2) are written so that every shell finds "centag".
func writeShim(L layout) error {
	if err := os.MkdirAll(L.binDir, 0o755); err != nil {
		return err
	}
	if isWindowsRuntime {
		// 1. .cmd shim for CMD / PowerShell.
		cmdContent := strings.ReplaceAll(renderWindowsShim(L.edition), "\n", "\r\n")
		if err := os.WriteFile(shimPath(L), []byte(cmdContent), 0o755); err != nil {
			return err
		}
		// 2. Bash shim for Git Bash / MSYS2 (no extension, executable).
		bashShim := filepath.Join(L.binDir, "centag")
		if err := os.WriteFile(bashShim, []byte(renderWindowsBashShim(L.edition)), 0o755); err != nil {
			return err
		}
		return os.Chmod(bashShim, 0o755)
	}
	if err := os.WriteFile(shimPath(L), []byte(renderUnixShim(L.edition)), 0o755); err != nil {
		return err
	}
	return os.Chmod(shimPath(L), 0o755)
}

func renderWindowsShim(edition string) string {
	return fmt.Sprintf(`@echo off
set ROOT=%%~dp0..
set EDITION=%[1]s
set LIB=%%ROOT%%\lib\%[1]s
set BIN=%%LIB%%\centag-%[1]s.exe
set CENTAG_EDITION=%%EDITION%%
if "%%STATIC_PATH%%"=="" set STATIC_PATH=%%LIB%%\static
if "%%PROJECT_ROOT%%"=="" set PROJECT_ROOT=%%LIB%%
if exist "%%LIB%%\config\profiles\%[1]s\initdata" if "%%INITDATA_PATH%%"=="" set INITDATA_PATH=%%LIB%%\config\profiles\%[1]s\initdata
"%%BIN%%" %%*
`, edition)
}

func renderUnixShim(edition string) string {
	return fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
EDITION=%[1]q
LIB="$ROOT/lib/$EDITION"
BIN="$LIB/centag-${EDITION}"
export CENTAG_EDITION="$EDITION"
export STATIC_PATH="${STATIC_PATH:-$LIB/static}"
# Seed data ships beside the binary (config/initdata, profile initdata).
export PROJECT_ROOT="${PROJECT_ROOT:-$LIB}"
if [[ -d "$LIB/config/profiles/$EDITION/initdata" ]]; then
  export INITDATA_PATH="${INITDATA_PATH:-$LIB/config/profiles/$EDITION/initdata}"
fi
[[ -x "$BIN" ]] || { echo "missing $BIN" >&2; exit 1; }
exec "$BIN" "$@"
`, edition)
}

// renderWindowsBashShim produces a bash shim for Git Bash / MSYS2 on Windows.
// It differs from the Unix shim by appending .exe to the binary path and
// ensuring USERPROFILE is set (Git Bash may not export it).
func renderWindowsBashShim(edition string) string {
	return fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
EDITION=%[1]q
LIB="$ROOT/lib/$EDITION"
BIN="$LIB/centag-${EDITION}.exe"
export CENTAG_EDITION="$EDITION"
export STATIC_PATH="${STATIC_PATH:-$LIB/static}"
export PROJECT_ROOT="${PROJECT_ROOT:-$LIB}"
# Ensure USERPROFILE is set for Windows .exe (Git Bash may not export it)
if [[ -z "${USERPROFILE:-}" ]]; then
  _win_home="$HOME"
  if [[ "$HOME" =~ ^/home/([^/]+)$ ]]; then
    _win_home="/c/Users/${BASH_REMATCH[1]}"
  fi
  export USERPROFILE="$(cygpath -w "$_win_home")"
fi
if [[ -d "$LIB/config/profiles/$EDITION/initdata" ]]; then
  export INITDATA_PATH="${INITDATA_PATH:-$LIB/config/profiles/$EDITION/initdata}"
fi
[[ -x "$BIN" ]] || { echo "missing $BIN" >&2; exit 1; }
exec "$BIN" "$@"
`, edition)
}

// installEditionLink creates bin/centag-<edition><ext> → lib binary. On unix
// this is a symlink (replaced if present); on windows it is best-effort
// (symlinks may be unavailable — the .cmd shim is the authoritative entry).
func installEditionLink(L layout) error {
	if L.edition == "" {
		return nil
	}
	link := filepath.Join(L.binDir, "centag-"+L.edition+L.ext)
	target := filepath.Join(L.editionLib, "centag-"+L.edition+L.ext)
	if samePath(link, target) {
		return nil
	}
	_ = os.Remove(link)
	if err := os.Symlink(target, link); err != nil {
		if isWindowsRuntime {
			return nil
		}
		return err
	}
	return nil
}

// writeEnvFiles writes the PATH helper files sourced by shell rc files
// (format identical to scripts/install.sh write_env_files — rewritten on
// every install/init so upgrades keep a single PATH snippet).
func writeEnvFiles(L layout) error {
	if err := os.MkdirAll(L.root, 0o755); err != nil {
		return err
	}
	bin := toSlash(L.binDir)
	env := fmt.Sprintf(`# Centag PATH helper — source after install, or from your shell rc:
#   source %[1]s/env
case ":$PATH:" in
  *":%[2]s:"*) ;;
  *) export PATH="%[2]s:$PATH" ;;
esac
hash -r 2>/dev/null || true
`, toSlash(L.root), bin)
	if err := os.WriteFile(filepath.Join(L.root, "env"), []byte(env), 0o644); err != nil {
		return err
	}
	envFish := fmt.Sprintf(`# Centag PATH helper for fish — source after install:
#   source %[1]s/env.fish
fish_add_path -g %[2]s
`, toSlash(L.root), bin)
	return os.WriteFile(filepath.Join(L.root, "env.fish"), []byte(envFish), 0o644)
}
