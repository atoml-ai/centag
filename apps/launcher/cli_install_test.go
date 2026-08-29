package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestShellQuoteSingle(t *testing.T) {
	if got := shellQuoteSingle("/Applications/Centag.app/Contents/Resources/centag-personal"); got != "/Applications/Centag.app/Contents/Resources/centag-personal" {
		t.Fatalf("plain path changed: %q", got)
	}
	if got := shellQuoteSingle("/tmp/it's/bin"); got != `/tmp/it'\''s/bin` {
		t.Fatalf("quote escaping wrong: %q", got)
	}
}

func TestPathContainsDir(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", "/usr/bin:"+dir+":/bin")
	if !pathContainsDir(dir) {
		t.Fatalf("pathContainsDir(%q) = false, want true", dir)
	}
	if pathContainsDir(filepath.Join(dir, "missing")) {
		t.Fatal("pathContainsDir(missing) = true, want false")
	}
}

func TestInstallCLIWindowsShim(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows-only shim layout")
	}
}

func TestInstallCentagCLIMissingBinary(t *testing.T) {
	if err := installCentagCLI(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Fatal("installCentagCLI(missing) = nil, want error")
	} else if !strings.Contains(err.Error(), "sidecar binary") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInstallCLILinuxFallback(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux-only symlink target")
	}
	bin := filepath.Join(t.TempDir(), "centag-personal")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	target := filepath.Join(home, ".local", "bin", "centag")
	if err := installCLILinuxFallback(bin); err != nil {
		t.Fatalf("installCLILinuxFallback: %v", err)
	}
	resolved, err := filepath.EvalSymlinks(target)
	if err != nil || resolved != bin {
		t.Fatalf("symlink %s -> %s (err=%v), want -> %s", target, resolved, err, bin)
	}
}
