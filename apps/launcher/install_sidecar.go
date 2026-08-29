package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// versionFileName marks the sidecar payload / installed lib version.
const versionFileName = "VERSION"

// centagInstallRoot returns the unified Centag home (default ~/.centag).
// Same root as scripts/install.sh / centag-layout.sh.
func centagInstallRoot() string {
	if v := strings.TrimSpace(os.Getenv("CENTAG_INSTALL_ROOT")); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".centag")
	}
	return filepath.Join(home, ".centag")
}

// editionLibDir is the per-edition home: <root>/lib/<edition>.
// Holds the installed sidecar (binary + static + config) and runtime data
// (storage/logs/data), matching the install.sh server layout.
func editionLibDir(edition Edition) string {
	return filepath.Join(centagInstallRoot(), "lib", string(edition))
}

func readVersionFile(dir string) string {
	b, err := os.ReadFile(filepath.Join(dir, versionFileName))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// compareVersions compares dotted versions segment-wise; segments compare by
// leading integer then lexically (so 0.2.7 < 0.2.10, 0.2.7-test > 0.2.7).
func compareVersions(a, b string) int {
	as := strings.Split(strings.TrimPrefix(a, "v"), ".")
	bs := strings.Split(strings.TrimPrefix(b, "v"), ".")
	for i := 0; i < len(as) || i < len(bs); i++ {
		var sa, sb string
		if i < len(as) {
			sa = as[i]
		}
		if i < len(bs) {
			sb = bs[i]
		}
		if c := compareVersionSegment(sa, sb); c != 0 {
			return c
		}
	}
	return 0
}

func compareVersionSegment(a, b string) int {
	an, ai := leadingInt(a)
	bn, bi := leadingInt(b)
	if an != bn {
		if an < bn {
			return -1
		}
		return 1
	}
	if c := strings.Compare(a[ai:], b[bi:]); c != 0 {
		return c
	}
	return 0
}

func leadingInt(s string) (int, int) {
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	n, _ := strconv.Atoi(s[:i])
	return n, i
}

// findPayloadDir locates the shipped sidecar payload: macOS app bundle
// Resources, or the directory next to the exe (windows/linux zip layout).
func findPayloadDir(exe string, edition Edition) string {
	if res := appBundleResourcesDir(exe); res != "" {
		if payloadBinaryPath(res, edition) != "" {
			return res
		}
		return ""
	}
	dir := filepath.Dir(exe)
	if payloadBinaryPath(dir, edition) != "" && dirExists(filepath.Join(dir, "static")) {
		return dir
	}
	return ""
}

func payloadBinaryPath(dir string, edition Edition) string {
	for _, name := range sidecarCandidateNames(edition) {
		p := filepath.Join(dir, name)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}

// ensureSidecarInstalled installs (or upgrades) the bundle payload into
// <root>/lib/<edition> and returns the installed sidecar binary path.
// Upgrade happens only when the payload VERSION is newer than the installed
// one, so re-launching the app never clobbers a newer server-managed install.
func ensureSidecarInstalled(payloadDir string, edition Edition) (string, error) {
	payloadBin := payloadBinaryPath(payloadDir, edition)
	if payloadBin == "" {
		return "", fmt.Errorf("no sidecar payload in %s", payloadDir)
	}
	libDir := editionLibDir(edition)
	if filepath.Clean(payloadDir) == filepath.Clean(libDir) {
		return payloadBin, nil
	}

	installedBin := filepath.Join(libDir, filepath.Base(payloadBin))
	payloadVer := readVersionFile(payloadDir)
	installedVer := readVersionFile(libDir)

	need := false
	switch {
	case !fileExists(installedBin):
		need = true
	case payloadVer == "":
		need = false // unversioned payload (dev): keep what's installed
	case installedVer == "":
		// No VERSION marker (e.g. installed by scripts/install.sh): probe the
		// binary itself so an older desktop payload cannot downgrade it.
		if v := binaryVersion(installedBin); v != "" {
			need = compareVersions(payloadVer, v) > 0
			if !need {
				// Adopt the marker so future launches compare by VERSION file.
				_ = os.WriteFile(filepath.Join(libDir, versionFileName), []byte(v+"\n"), 0o644)
			}
		} else {
			need = true
		}
	default:
		need = compareVersions(payloadVer, installedVer) > 0
	}
	if !need {
		return installedBin, nil
	}
	if err := installSidecarTree(payloadDir, libDir, payloadBin, payloadVer); err != nil {
		return "", err
	}
	return installedBin, nil
}

// runCommand executes a command with a timeout and returns combined output.
func runCommand(timeout time.Duration, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	return buf.String(), err
}

// binaryVersion runs `bin version` and parses the "centag vX.Y.Z" line.
// Best-effort: empty when the probe fails or the binary predates the command.
func binaryVersion(bin string) string {
	out, err := runCommand(5*time.Second, bin, "version")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if rest, ok := strings.CutPrefix(line, "centag v"); ok {
			return strings.TrimSpace(rest)
		}
	}
	return ""
}

// installSidecarTree copies binary + static + config from the payload into
// libDir (mirrors install.sh install_sidecar_tree_into_lib). Runtime data
// (storage/logs/data) is never touched.
func installSidecarTree(srcDir, dstDir, payloadBin, version string) error {
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return err
	}

	// Binary: copy to temp then rename — atomic and safe even when another
	// instance is executing the old binary.
	dstBin := filepath.Join(dstDir, filepath.Base(payloadBin))
	tmpBin := dstBin + ".new"
	if err := copyFile(payloadBin, tmpBin); err != nil {
		return fmt.Errorf("copy sidecar binary: %w", err)
	}
	if err := os.Chmod(tmpBin, 0o755); err != nil {
		_ = os.Remove(tmpBin)
		return err
	}
	if err := os.Rename(tmpBin, dstBin); err != nil {
		_ = os.Remove(tmpBin)
		return fmt.Errorf("replace sidecar binary: %w", err)
	}

	for _, sub := range []string{"static", "config"} {
		src := filepath.Join(srcDir, sub)
		if !dirExists(src) {
			continue
		}
		dst := filepath.Join(dstDir, sub)
		if err := os.RemoveAll(dst); err != nil {
			return err
		}
		if err := copyTree(src, dst); err != nil {
			return fmt.Errorf("install %s: %w", sub, err)
		}
	}

	if version != "" {
		if err := os.WriteFile(filepath.Join(dstDir, versionFileName), []byte(version+"\n"), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if !d.Type().IsRegular() {
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return copyFile(path, target)
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	st, err := in.Stat()
	if err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, st.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

func dirExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.IsDir()
}

func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}
