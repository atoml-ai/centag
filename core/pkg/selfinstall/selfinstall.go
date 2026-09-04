// Package selfinstall implements the `centag install` / `centag uninstall`
// subcommands: the binary deploys itself into the install layout shared with
// scripts/install.sh and scripts/lib/centag-layout.sh (default root ~/.centag):
//
//	<root>/bin/centag(.cmd)       PATH shim (wrapper contract = install.sh)
//	<root>/bin/centag-<edition>   symlink to the lib binary (unix)
//	<root>/lib/<edition>/         binary + static/ + config/ sidecar tree
//	<root>/env, <root>/env.fish   PATH helper sourced by shell rc files
//
// PATH persistence: POSIX shell rc files (sourcing the env helper), Windows
// HKCU Environment\Path + WM_SETTINGCHANGE broadcast. Administrator
// privileges are never required.
package selfinstall

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Options carries parsed CLI flags for init / uninstall.
type Options struct {
	// Root overrides the install root (default: CENTAG_INSTALL_ROOT or ~/.centag).
	Root string
	// BinDir overrides the PATH directory (default: CENTAG_BIN_DIR or <root>/bin).
	BinDir string
	// NoModifyPath skips rc file / registry PATH persistence.
	NoModifyPath bool
	// Stdout / Stderr receive progress output (default os.Stdout / os.Stderr).
	Stdout io.Writer
	Stderr io.Writer
}

type layout struct {
	root       string
	binDir     string
	libDir     string
	editionLib string
	edition    string
	ext        string // ".exe" on windows, else ""
}

func (o Options) out() io.Writer {
	if o.Stdout != nil {
		return o.Stdout
	}
	return os.Stdout
}

func (o Options) errOut() io.Writer {
	if o.Stderr != nil {
		return o.Stderr
	}
	return os.Stderr
}

func (o Options) logf(format string, args ...any) {
	fmt.Fprintf(o.out(), format+"\n", args...)
}

func (o Options) warnf(format string, args ...any) {
	fmt.Fprintf(o.errOut(), "warn: "+format+"\n", args...)
}

// RunInit sets up the runtime environment: PATH shim, env helpers, and PATH
// persistence (registry on Windows, shell rc on Unix). It does NOT copy
// binaries or sidecar files — the binary is expected to already be in place
// (built by `make build` or installed by a package manager).
func RunInit(opts Options) error {
	L, err := resolveLayout(opts)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(L.binDir, 0o755); err != nil {
		return fmt.Errorf("create bin dir: %w", err)
	}

	// 1. PATH shim + edition symlink in bin/.
	if err := writeShim(L); err != nil {
		return fmt.Errorf("write shim: %w", err)
	}
	opts.logf("shim    → %s", shimPath(L))
	if err := installEditionLink(L); err != nil {
		opts.warnf("edition link: %v", err)
	}

	// 2. env helper files (sourced by rc files; rewritten on every init).
	if err := writeEnvFiles(L); err != nil {
		return fmt.Errorf("write env helper: %w", err)
	}

	// 3. PATH persistence.
	if opts.NoModifyPath {
		opts.logf("PATH untouched (--no-modify-path)")
	} else {
		res, err := appendBinDirToPath(L.root, L.binDir)
		if err != nil {
			opts.warnf("PATH persistence failed (%v) — add manually:", err)
			opts.warnf("  export PATH=\"%s:$PATH\"", toSlash(L.binDir))
		} else if res.changed {
			opts.logf("PATH    → %s", res.detail)
		} else {
			opts.logf("PATH    → already configured (%s)", res.detail)
		}
	}

	opts.logf("\nCentag init complete (edition: %s).", L.edition)
	printActivateHint(opts, L)
	return nil
}

// RunUninstall removes all artifacts created by RunInit: shims, env helpers,
// and PATH entries (registry on Windows, shell rc on Unix).
func RunUninstall(opts Options) error {
	L, err := resolveLayout(opts)
	if err != nil {
		return err
	}

	// 1. bin/ entries (shim + edition link).
	removed := 0
	for _, p := range binEntryPaths(L) {
		if err := os.Remove(p); err == nil {
			removed++
			opts.logf("removed → %s", p)
		}
	}
	if removed == 0 {
		opts.logf("no bin entries found under %s", L.binDir)
	}

	// 2. env helper files.
	for _, name := range []string{"env", "env.fish"} {
		p := filepath.Join(L.root, name)
		if err := os.Remove(p); err == nil {
			opts.logf("removed → %s", p)
		}
	}

	// 3. PATH persistence.
	if opts.NoModifyPath {
		opts.logf("PATH untouched (--no-modify-path)")
	} else {
		res, err := removeBinDirFromPath(L.root, L.binDir)
		if err != nil {
			opts.warnf("PATH cleanup failed: %v", err)
		} else if res.changed {
			opts.logf("PATH    → removed %s (%s)", toSlash(L.binDir), res.detail)
		} else {
			opts.logf("PATH    → no entry found (%s)", res.detail)
		}
	}

	opts.logf("\nCentag uninstall complete.")
	return nil
}

func printActivateHint(opts Options, L layout) {
	w := opts.out()
	fmt.Fprintf(w, "  Install root: %s\n", L.root)
	fmt.Fprintf(w, "  Bin dir:      %s\n", L.binDir)
	if !opts.NoModifyPath {
		fmt.Fprintln(w)
		if isWindowsRuntime {
			fmt.Fprintf(w, "  Open a NEW terminal so the updated user PATH is picked up.\n")
			fmt.Fprintf(w, "  For CMD / PowerShell:\n")
			fmt.Fprintf(w, "    set PATH=%s;%%PATH%%\n", L.binDir)
			fmt.Fprintf(w, "  For Git Bash / MSYS2:\n")
			fmt.Fprintf(w, "    source \"%s/env\"\n", toSlash(L.root))
			return
		}
		fmt.Fprintf(w, "  Activate now (or open a new terminal):\n")
		fmt.Fprintf(w, "    source \"%s/env\"\n", toSlash(L.root))
	}
}

// binEntryPaths lists bin/ files managed by init/uninstall (existing or not).
// Only shims created by init are listed — edition binaries placed by
// make build / install scripts are not touched.
func binEntryPaths(L layout) []string {
	paths := []string{shimPath(L)}
	if isWindowsRuntime {
		// Also list the bash shim for Git Bash / MSYS2 cleanup.
		paths = append(paths, filepath.Join(L.binDir, "centag"))
	}
	return paths
}

// ParseOptions parses init/uninstall flags. help is set when -h/--help was
// requested. Positional arguments are rejected: these commands are flag-only.
func ParseOptions(args []string) (opts Options, help bool, err error) {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-h" || arg == "--help":
			help = true
		case arg == "--no-modify-path":
			opts.NoModifyPath = true
		case arg == "--prefix":
			if i+1 >= len(args) {
				return opts, false, fmt.Errorf("--prefix requires a directory")
			}
			i++
			opts.Root = strings.TrimSpace(args[i])
		case arg == "--bin-dir":
			if i+1 >= len(args) {
				return opts, false, fmt.Errorf("--bin-dir requires a directory")
			}
			i++
			opts.BinDir = strings.TrimSpace(args[i])
		case strings.HasPrefix(arg, "-"):
			return opts, false, fmt.Errorf("unknown flag %q (supported: --prefix, --bin-dir, --no-modify-path, --help)", arg)
		default:
			return opts, false, fmt.Errorf("unexpected argument %q (init/uninstall take no positional arguments)", arg)
		}
	}
	return opts, help, nil
}

// PrintInstallUsage writes the install/uninstall flag help.
func PrintInstallUsage(w io.Writer) {
	fmt.Fprint(w, `centag install — set up PATH, shims and env helpers for this binary

Usage:
  centag install [--prefix <dir>] [--bin-dir <dir>] [--no-modify-path]
  centag uninstall [--prefix <dir>] [--bin-dir <dir>] [--no-modify-path]

What install does:
  1. writes the PATH shim <root>/bin/centag(.cmd) and the edition symlink
  2. writes <root>/env and <root>/env.fish PATH helpers
  3. persists PATH: shell rc file (unix) or user registry (windows)

Options:
  --prefix <dir>       Install root (default: CENTAG_INSTALL_ROOT or ~/.centag)
  --bin-dir <dir>      PATH directory (default: <root>/bin)
  --no-modify-path     Do not touch rc files / registry
  -h, --help           Show this help

uninstall removes the shim, symlink, env helpers and PATH entries.
`)
}
