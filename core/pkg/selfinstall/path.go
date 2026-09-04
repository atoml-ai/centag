package selfinstall

// pathResult reports what PATH persistence did (per-OS implementations in
// path_unix.go / path_windows.go).
type pathResult struct {
	changed bool
	detail  string // where the entry lives, or why it was left untouched
}
