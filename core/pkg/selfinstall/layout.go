package selfinstall

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

var isWindowsRuntime = runtime.GOOS == "windows"

// resolveLayout binds the install paths for the current binary/edition.
func resolveLayout(opts Options) (layout, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return layout{}, fmt.Errorf("resolve home: %w", err)
	}
	root := opts.Root
	if root == "" {
		root = strings.TrimSpace(os.Getenv("CENTAG_INSTALL_ROOT"))
	}
	if root == "" {
		root = filepath.Join(home, ".centag")
	}
	root = filepath.Clean(abs(root))

	binDir := opts.BinDir
	if binDir == "" {
		binDir = strings.TrimSpace(os.Getenv("CENTAG_BIN_DIR"))
	}
	if binDir == "" {
		binDir = filepath.Join(root, "bin")
	}
	binDir = filepath.Clean(abs(binDir))

	L := layout{
		root:    root,
		binDir:  binDir,
		libDir:  filepath.Join(root, "lib"),
		edition: detectEdition(),
		ext:     exeExt(),
	}
	L.editionLib = filepath.Join(L.libDir, L.edition)
	return L, nil
}

func abs(p string) string {
	if a, err := filepath.Abs(p); err == nil {
		return a
	}
	return p
}

func exeExt() string {
	if isWindowsRuntime {
		return ".exe"
	}
	return ""
}

var editionRe = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

// detectEdition resolves the edition of the running binary:
// CENTAG_EDITION env → own file name (centag-<edition>) → "personal".
func detectEdition() string {
	if e := editionFromName(os.Getenv("CENTAG_EDITION")); e != "" {
		return e
	}
	if exe, err := os.Executable(); err == nil {
		base := strings.TrimSuffix(filepath.Base(exe), ".exe")
		if rest, ok := strings.CutPrefix(base, "centag-"); ok {
			if e := editionFromName(rest); e != "" {
				return e
			}
		}
	}
	return "personal"
}

// editionFromName sanitizes a candidate edition label; "" when unusable.
func editionFromName(name string) string {
	e := strings.ToLower(strings.TrimSpace(name))
	if e == "" || !editionRe.MatchString(e) {
		return ""
	}
	return e
}

// samePath compares two paths after symlink resolution and cleaning
// (case-insensitive on windows).
func samePath(a, b string) bool {
	for _, p := range []*string{&a, &b} {
		if resolved, err := filepath.EvalSymlinks(*p); err == nil {
			*p = resolved
		}
		*p = abs(*p)
	}
	if isWindowsRuntime {
		return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
	}
	return filepath.Clean(a) == filepath.Clean(b)
}

// toSlash converts to forward slashes for POSIX-side artifacts (env helpers,
// rc lines): native unix paths are unchanged, MSYS/git-bash accept C:/ style.
func toSlash(p string) string {
	return filepath.ToSlash(p)
}
