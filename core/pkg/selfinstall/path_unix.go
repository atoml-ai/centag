//go:build !windows

package selfinstall

// appendBinDirToPath persists binDir for future POSIX shells by appending a
// marked "source <root>/env" block to the shell rc file (same contract as
// scripts/install.sh ensure_path / add_to_path).
func appendBinDirToPath(root, binDir string) (pathResult, error) {
	return AppendBashRcPath(root, binDir)
}

// removeBinDirFromPath strips centag-added PATH blocks and legacy inline
// entries from all known rc files. Reports how many lines were removed.
func removeBinDirFromPath(root, binDir string) (pathResult, error) {
	return RemoveBashRcPath(root, binDir)
}
