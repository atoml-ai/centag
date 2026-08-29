package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"0.2.7", "0.2.7", 0},
		{"0.2.10", "0.2.7", 1},
		{"0.2.7", "0.2.10", -1},
		{"0.3.0", "0.2.9", 1},
		{"1.0", "1.0.0", 0},
		{"0.2.8-test", "0.2.7", 1},
		{"v0.2.8", "0.2.7", 1},
	}
	for _, c := range cases {
		if got := compareVersions(c.a, c.b); got != c.want {
			t.Fatalf("compareVersions(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestFindPayloadDir(t *testing.T) {
	root := t.TempDir()
	res := filepath.Join(root, "Centag.app", "Contents", "Resources")
	macOS := filepath.Join(root, "Centag.app", "Contents", "MacOS")
	for _, d := range []string{res, macOS} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(res, "centag-personal"), []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(macOS, "Centag")
	if got := findPayloadDir(exe, EditionPersonal); got != res {
		t.Fatalf("findPayloadDir(bundle) = %q, want %q", got, res)
	}
	// No payload → empty.
	if got := findPayloadDir(filepath.Join(root, "plain", "exe"), EditionPersonal); got != "" {
		t.Fatalf("findPayloadDir(plain) = %q, want empty", got)
	}
	// Zip layout (windows/linux): payload + static beside the exe.
	zipDir := filepath.Join(root, "Centag")
	if err := os.MkdirAll(filepath.Join(zipDir, "static"), 0o755); err != nil {
		t.Fatal(err)
	}
	payloadName := sidecarCandidateNames(EditionPersonal)[0]
	if err := os.WriteFile(filepath.Join(zipDir, payloadName), []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	zipExe := filepath.Join(zipDir, "Centag.exe")
	if got := findPayloadDir(zipExe, EditionPersonal); got != zipDir {
		t.Fatalf("findPayloadDir(zip) = %q, want %q", got, zipDir)
	}
}

func TestEnsureSidecarInstalled(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CENTAG_INSTALL_ROOT", root)

	payload := filepath.Join(root, "payload")
	if err := os.MkdirAll(filepath.Join(payload, "static"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(payload, "config", "initdata"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeBin := func(dir, marker string) {
		if err := os.WriteFile(filepath.Join(dir, "centag-personal"), []byte(marker), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeVersion := func(dir, v string) {
		if err := os.WriteFile(filepath.Join(dir, "VERSION"), []byte(v+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeBin(payload, "v1")
	writeVersion(payload, "0.2.7")

	lib := editionLibDir(EditionPersonal)

	// First install.
	got, err := ensureSidecarInstalled(payload, EditionPersonal)
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(lib, "centag-personal") {
		t.Fatalf("installed path = %q", got)
	}
	bin := readBin(t, got)
	if bin != "v1" {
		t.Fatalf("installed binary marker = %q, want v1", bin)
	}
	if !dirExists(filepath.Join(lib, "static")) || !dirExists(filepath.Join(lib, "config", "initdata")) {
		t.Fatal("static/config not installed")
	}
	if readVersionFile(lib) != "0.2.7" {
		t.Fatalf("installed VERSION = %q", readVersionFile(lib))
	}

	// Same version → no churn.
	if err := os.WriteFile(filepath.Join(lib, "static", "keep.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ensureSidecarInstalled(payload, EditionPersonal); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(lib, "static", "keep.txt")); err != nil {
		t.Fatal("same-version install churned static/")
	}

	// Newer payload → upgrade (binary replaced atomically, runtime dirs untouched).
	writeBin(payload, "v2")
	writeVersion(payload, "0.2.8")
	got, err = ensureSidecarInstalled(payload, EditionPersonal)
	if err != nil {
		t.Fatal(err)
	}
	if readBin(t, got) != "v2" {
		t.Fatal("upgrade did not replace binary")
	}
	if readVersionFile(lib) != "0.2.8" {
		t.Fatalf("installed VERSION after upgrade = %q", readVersionFile(lib))
	}

	// Older payload → downgrade refused.
	writeBin(payload, "v0")
	writeVersion(payload, "0.2.1")
	if _, err := ensureSidecarInstalled(payload, EditionPersonal); err != nil {
		t.Fatal(err)
	}
	if readBin(t, got) != "v2" {
		t.Fatal("downgrade replaced binary")
	}

	// Older payload vs marker-less (install.sh-style) lib: binary probe prevents downgrade.
	legacy := t.TempDir()
	legacyBin := filepath.Join(legacy, "centag-personal")
	if err := os.WriteFile(legacyBin, []byte("#!/bin/sh\necho 'centag v9.9.9'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CENTAG_INSTALL_ROOT", legacy)
	lib2 := editionLibDir(EditionPersonal)
	if err := os.MkdirAll(lib2, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := copyFile(legacyBin, filepath.Join(lib2, "centag-personal")); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(lib2, "centag-personal"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err = ensureSidecarInstalled(payload, EditionPersonal)
	if err != nil {
		t.Fatal(err)
	}
	if readBin(t, got) != "#!/bin/sh\necho 'centag v9.9.9'\n" {
		t.Fatal("older payload downgraded a newer server-managed binary")
	}
	if readVersionFile(lib2) != "9.9.9" {
		t.Fatalf("VERSION marker not adopted: %q", readVersionFile(lib2))
	}
}

func readBin(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
