package internal

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestInstallRootFromExecPath(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		exec string
		want string
	}{
		{name: "bin layout", exec: "/app/bin/centag", want: "/app"},
		{name: "flat layout", exec: "/opt/centag/centag", want: "/opt/centag"},
		{name: "fnos", exec: "/vol/apps/centag/bin/centag", want: "/vol/apps/centag"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := installRootFromExecPath(tc.exec)
			if got != tc.want {
				t.Fatalf("installRootFromExecPath(%q)=%q want %q", tc.exec, got, tc.want)
			}
		})
	}
}

func TestRemapUpdateTarget(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Server bin/ layout: the process itself is bin/centag → keep bin/ mapping.
	currentExecutableNameOverride = "centag"
	if got := remapUpdateTarget(root, "centag"); got != filepath.Join("bin", "centag") {
		t.Fatalf("with bin/: got %q", got)
	}
	if got := remapUpdateTarget(root, "bin/centag"); got != filepath.Join("bin", "centag") {
		t.Fatalf("already bin/: got %q", got)
	}
	currentExecutableNameOverride = ""

	if got := remapUpdateTarget(root, "static/"); got != "static" {
		t.Fatalf("static: got %q want static", got)
	}

	// Desktop/edition layout: running binary is centag-personal, so the
	// generic manifest name must map onto the running process name.
	currentExecutableNameOverride = "centag-personal"
	if got := remapUpdateTarget(root, "centag"); got != "centag-personal" {
		t.Fatalf("edition-named process: got %q want centag-personal", got)
	}
	if runtime.GOOS == "windows" {
		currentExecutableNameOverride = "centag-personal.exe"
		if got := remapUpdateTarget(root, "centag.exe"); got != "centag-personal.exe" {
			t.Fatalf("windows edition: got %q", got)
		}
	}
	currentExecutableNameOverride = ""
}
