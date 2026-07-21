package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
)

// projectRootCache caches the computed project root to avoid repeated os.Executable() calls.
var projectRootCache string

// ProjectRoot returns the project root directory.
//
// Resolution order (deterministic, no fallbacks):
//  1. PROJECT_ROOT environment variable (absolute path to project root)
//  2. Parent when the executable resides under bin/ or var/bin/
//     (legacy: ./bin/server/centag → ../../)
//  3. Directory containing the executable itself
//     (install/dev layout: ~/.centag/lib/<edition>/centag-<edition>;
//     install wrapper sets PROJECT_ROOT to that lib dir)
//
// For development with "go run", set PROJECT_ROOT or INITDATA_PATH explicitly.
func ProjectRoot() string {
	// PROJECT_ROOT env always wins (tests and go run set this explicitly).
	if p := strings.TrimSpace(os.Getenv("PROJECT_ROOT")); p != "" {
		return p
	}

	if projectRootCache != "" {
		return projectRootCache
	}

	// 2. Derive from executable location
	execPath, err := os.Executable()
	if err != nil {
		// Cannot determine executable path; return empty string.
		// Callers should handle missing paths gracefully.
		return ""
	}
	execDir := filepath.Dir(execPath)

	// Walk up to find bin/ or var/bin/, then return its parent as project root.
	dir := execDir
	for {
		base := filepath.Base(dir)
		if base == "bin" {
			parent := filepath.Dir(dir)
			if filepath.Base(parent) == "var" {
				projectRootCache = filepath.Dir(parent)
			} else {
				projectRootCache = parent
			}
			return projectRootCache
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	// Otherwise, project root is the directory containing the executable.
	projectRootCache = execDir
	return projectRootCache
}

// InitdataRoot returns the root directory for initdata files.
//
// Resolution order (deterministic, no fallbacks):
//  1. INITDATA_PATH environment variable
//  2. ProjectRoot() + "/config/initdata"
func InitdataRoot() string {
	// 1. Explicit override
	if p := os.Getenv("INITDATA_PATH"); p != "" {
		return p
	}

	// 2. Convention: config/initdata/ under project root
	root := ProjectRoot()
	if root == "" {
		return ""
	}
	return filepath.Join(root, "config", "initdata")
}

// InitdataRoots returns both the global and profile-specific initdata roots.
//
// When INITDATA_PATH is set (e.g. in Docker profiles), it returns:
//   - global: ProjectRoot()/config/initdata (pipeline templates / customer zip fallback)
//   - profile: INITDATA_PATH (edition/customer seed; preferred for initial-backends)
//
// When INITDATA_PATH is not set, both paths point to the same directory.
// Backend seed loading is profile-first and does not union-merge with global.
func InitdataRoots() (global, profile string) {
	profile = InitdataRoot()
	global = filepath.Join(ProjectRoot(), "config", "initdata")
	return global, profile
}

// InitdataPath returns the full filesystem path for a file or directory
// located under the initdata root. If InitdataRoot() cannot be determined,
// it returns the subpath unchanged.
func InitdataPath(subpath string) string {
	root := InitdataRoot()
	if root == "" {
		return subpath
	}
	return filepath.Join(root, subpath)
}
