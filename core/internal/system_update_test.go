package internal

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestRemapUpdateTarget_MapsToRunningProcessName(t *testing.T) {
	root := t.TempDir() // no bin/ subdir → not the server layout
	got := remapUpdateTarget(root, "centag")
	exe := currentExecutableName()
	if exe == "" {
		t.Skip("os.Executable unavailable")
	}
	if exe != "centag" && exe != "centag.exe" {
		// The deployed binary is centag-personal[.exe]: the generic manifest
		// name must map onto the running process name, otherwise the update
		// lands on a dead filename and never takes effect.
		if got != exe {
			t.Fatalf("remapUpdateTarget(centag) = %q, want %q", got, exe)
		}
	}
	// Windows-flavored generic name maps the same way.
	if runtime.GOOS == "windows" {
		if got := remapUpdateTarget(root, "centag.exe"); got != exe {
			t.Fatalf("remapUpdateTarget(centag.exe) = %q, want %q", got, exe)
		}
	}
}

func TestRemapUpdateTarget_ServerLayoutKept(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	// When the running process IS the generic name (bin/centag server layout),
	// the bin/ mapping must win.
	if currentExecutableName() == "centag" {
		if got := remapUpdateTarget(root, "centag"); got != filepath.Join("bin", "centag") {
			t.Fatalf("got %q", got)
		}
	}
	// Non-binary targets pass through untouched.
	if got := remapUpdateTarget(root, "static/"); got != "static" {
		t.Fatalf("static remap changed: %q", got)
	}
	if got := remapUpdateTarget(root, "./data/x.yml"); got != filepath.Join("data", "x.yml") {
		t.Fatalf("nested remap changed: %q", got)
	}
}

func TestReplaceBinaryFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "new-bin")
	dst := filepath.Join(dir, "centag-personal")
	if err := os.WriteFile(src, []byte("NEW"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte("OLD"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := replaceBinaryFile(src, dst); err != nil {
		t.Fatalf("replace: %v", err)
	}
	got, err := os.ReadFile(dst)
	if err != nil || string(got) != "NEW" {
		t.Fatalf("dst content = %q err=%v", got, err)
	}
	// Source and stale .tmp/.old temp names must not linger.
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".old" {
			t.Errorf("unexpected leftover %s", e.Name())
		}
	}

	// Second replace over the just-replaced file (repeatability).
	if err := replaceBinaryFile(src, dst); err != nil {
		t.Fatalf("second replace: %v", err)
	}
	if got, _ := os.ReadFile(dst); string(got) != "NEW" {
		t.Fatalf("second replace content = %q", got)
	}
}

func TestValidateExecutableForHost_RejectsForeignBinary(t *testing.T) {
	dir := t.TempDir()
	switch runtime.GOOS {
	case "darwin":
		p := filepath.Join(dir, "b")
		if err := os.WriteFile(p, []byte{0x7f, 'E', 'L', 'F', 0, 0}, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := validateExecutableForHost(p); err == nil {
			t.Fatal("ELF should be rejected on darwin")
		}
	case "windows":
		p := filepath.Join(dir, "b")
		if err := os.WriteFile(p, []byte{0x7f, 'E', 'L', 'F', 0, 0}, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := validateExecutableForHost(p); err == nil {
			t.Fatal("ELF should be rejected on windows")
		}
	default:
		p := filepath.Join(dir, "b")
		if err := os.WriteFile(p, []byte{0xcf, 0xfa, 0xed, 0xfe, 0, 0}, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := validateExecutableForHost(p); err == nil {
			t.Fatal("Mach-O should be rejected on linux")
		}
	}
}
