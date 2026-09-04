//go:build windows

package selfinstall

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows/registry"
)

// readRawUserPath reads HKCU\Environment\Path straight from the registry
// ("" when the value does not exist) to verify persistence independently of
// the code under test.
func readRawUserPath(t *testing.T) string {
	t.Helper()
	k, err := registry.OpenKey(registry.CURRENT_USER, `Environment`, registry.QUERY_VALUE)
	if err != nil {
		t.Fatalf("open Environment: %v", err)
	}
	defer k.Close()
	val, _, err := k.GetStringValue(envPathValue)
	if err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return ""
		}
		t.Fatalf("read Path: %v", err)
	}
	return val
}

// TestUserPathRoundTrip exercises the real HKCU\Environment\Path append and
// remove cycle with a throwaway directory (added, verified, removed —
// t.Cleanup guarantees removal even on failure).
func TestUserPathRoundTrip(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "bin")

	add, err := appendBinDirToPath("", dir)
	if err != nil {
		t.Fatalf("append: %v", err)
	}
	if !add.changed {
		t.Fatalf("append should change PATH on first call, detail: %s", add.detail)
	}
	t.Cleanup(func() {
		if _, err := removeBinDirFromPath("", dir); err != nil {
			t.Errorf("cleanup remove: %v", err)
		}
	})

	val := readRawUserPath(t)
	if !pathListContains(val, dir) {
		t.Fatalf("PATH does not contain %q after append: %q", dir, val)
	}

	// Second append is a no-op (dedup).
	again, err := appendBinDirToPath("", dir)
	if err != nil {
		t.Fatalf("second append: %v", err)
	}
	if again.changed {
		t.Errorf("second append should be a no-op, detail: %s", again.detail)
	}

	// Remove restores the previous list.
	rem, err := removeBinDirFromPath("", dir)
	if err != nil {
		t.Fatalf("remove: %v", err)
	}
	if !rem.changed {
		t.Errorf("remove should report change, detail: %s", rem.detail)
	}
	val = readRawUserPath(t)
	if pathListContains(val, dir) {
		t.Errorf("PATH still contains %q after remove: %q", dir, val)
	}

	// Removing again is a no-op.
	rem2, err := removeBinDirFromPath("", dir)
	if err != nil {
		t.Fatalf("second remove: %v", err)
	}
	if rem2.changed {
		t.Errorf("second remove should be a no-op, detail: %s", rem2.detail)
	}
}

// TestSameWinPath covers the case-insensitive comparison used for dedup.
func TestSameWinPath(t *testing.T) {
	if !sameWinPath(`C:\Users\A\.centag\bin`, `c:\users\a\.centag\bin\`) {
		t.Fatal("same path with different case / trailing sep should match")
	}
	if sameWinPath(`C:\Users\A\.centag\bin`, `C:\Users\A\.centag\lib`) {
		t.Fatal("different paths should not match")
	}
	if _, err := os.Stat(`C:\`); err != nil {
		t.Log("unexpected: no C:\\ drive")
	}
}
