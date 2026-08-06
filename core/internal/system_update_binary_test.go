package internal

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestValidateExecutableForHost_RejectsWrongMagic(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	bad := filepath.Join(dir, "centag")
	// Fake Mach-O header on all platforms; on darwin this is "valid", so skip reject check there.
	if err := os.WriteFile(bad, []byte{0xcf, 0xfa, 0xed, 0xfe, 0x00, 0x00}, 0o755); err != nil {
		t.Fatal(err)
	}
	err := validateExecutableForHost(bad)
	switch runtime.GOOS {
	case "linux":
		if err == nil {
			t.Fatal("expected linux to reject Mach-O payload")
		}
	case "darwin":
		if err != nil {
			t.Fatalf("darwin should accept Mach-O: %v", err)
		}
	}
}

func TestValidateExecutableForHost_AcceptsELFOnLinux(t *testing.T) {
	t.Parallel()
	if runtime.GOOS != "linux" {
		t.Skip("ELF accept path is linux-only")
	}
	dir := t.TempDir()
	elf := filepath.Join(dir, "centag")
	if err := os.WriteFile(elf, []byte{0x7f, 'E', 'L', 'F', 0x02}, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := validateExecutableForHost(elf); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}
