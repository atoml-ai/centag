package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestDetectInstalled covers TC-DET-001/002.
func TestDetectInstalled(t *testing.T) {
	dir := t.TempDir()
	name := "centag-test-bin"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	bin := filepath.Join(dir, name)
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	t.Run("TC-DET-001", func(t *testing.T) {
		p, ok := detectInstalled(installSpec{CLIBinaries: []string{name}})
		if !ok || p == "" {
			t.Fatalf("expected installed, got ok=%v path=%q", ok, p)
		}
	})

	t.Run("TC-DET-002", func(t *testing.T) {
		if _, ok := detectInstalled(installSpec{CLIBinaries: []string{"centag-definitely-not-installed-zzz"}}); ok {
			t.Fatal("expected not installed")
		}
	})
}

// TestDetectInstalledPlatform covers TC-DET-003 (macOS .app) / TC-DET-004
// (Windows exe search dirs); skipped on other platforms.
func TestDetectInstalledPlatform(t *testing.T) {
	switch runtime.GOOS {
	case "darwin":
		t.Run("TC-DET-003", func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			app := filepath.Join(home, "Applications", "Foo.app")
			if err := os.MkdirAll(app, 0o755); err != nil {
				t.Fatal(err)
			}
			p, ok := detectInstalled(installSpec{MacApps: []string{"Foo"}})
			if !ok || p != app {
				t.Fatalf("got %q ok=%v want %q", p, ok, app)
			}
		})
	case "windows":
		t.Run("TC-DET-004", func(t *testing.T) {
			dir := t.TempDir()
			exe := filepath.Join(dir, "centag-widget.exe")
			if err := os.WriteFile(exe, []byte("x"), 0o755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("ProgramFiles", dir)
			p, ok := detectInstalled(installSpec{WinExes: []string{"centag-widget.exe"}})
			if !ok || p != exe {
				t.Fatalf("got %q ok=%v want %q", p, ok, exe)
			}
		})
		t.Run("TC-DET-008 MSIX", func(t *testing.T) {
			local := t.TempDir()
			pkg := filepath.Join(local, "Packages", "OpenAI.Codex_2p2nqsd0c76g0")
			if err := os.MkdirAll(pkg, 0o755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("LOCALAPPDATA", local)
			p, ok := detectInstalled(installSpec{WinMSIX: []string{"OpenAI.Codex_"}})
			if !ok || p != "OpenAI.Codex_2p2nqsd0c76g0!App" {
				t.Fatalf("got %q ok=%v want AUMID", p, ok)
			}
			if _, ok := detectInstalled(installSpec{WinMSIX: []string{"Centag.DefinitelyNot_"}}); ok {
				t.Fatal("unexpected MSIX match")
			}
		})
	}
}

// TestMatchInstalledApp covers TC-DET-007: desktop aliases match the local
// installed-app index (case-insensitive substring).
func TestMatchInstalledApp(t *testing.T) {
	prev := appIndexProvider
	t.Cleanup(func() { appIndexProvider = prev })
	appIndexProvider = func() []installedApp {
		return []installedApp{
			{Name: "OpenCode", Path: "/Applications/OpenCode.app"},
			{Name: "WorkBuddy", Path: "C:/WorkBuddy/WorkBuddy.exe"},
		}
	}

	t.Run("TC-DET-007", func(t *testing.T) {
		if p, ok := matchInstalledApp([]string{"opencode"}); !ok || p != "/Applications/OpenCode.app" {
			t.Fatalf("alias opencode => %q ok=%v", p, ok)
		}
		if p, ok := matchInstalledApp([]string{"WorkBuddy"}); !ok || p != "C:/WorkBuddy/WorkBuddy.exe" {
			t.Fatalf("alias WorkBuddy => %q ok=%v", p, ok)
		}
		if _, ok := matchInstalledApp([]string{"no-such-app-xyz"}); ok {
			t.Fatal("unexpected match")
		}
	})
}

// TestRunApps covers list rendering and --installed filtering with an injected
// offline catalog (no sidecar / network).
func TestRunApps(t *testing.T) {
	prev := appCatalogJSON
	t.Cleanup(func() { appCatalogJSON = prev })
	SetAppCatalogJSON(func() ([]byte, error) {
		return []byte(`{"apps":[{"id":"opencode","display_name":"OpenCode","launch_mode":"wrap_run","install":{"cli_binaries":["centag-definitely-not-installed-zzz"]}}]}`), nil
	})

	t.Run("table", func(t *testing.T) {
		var buf bytes.Buffer
		if err := runAppsTo(&buf, nil); err != nil {
			t.Fatal(err)
		}
		out := buf.String()
		if !strings.Contains(out, "opencode") || !strings.Contains(out, "0/1 installed") {
			t.Fatalf("unexpected output:\n%s", out)
		}
	})

	t.Run("json", func(t *testing.T) {
		var buf bytes.Buffer
		if err := runAppsTo(&buf, []string{"--json"}); err != nil {
			t.Fatal(err)
		}
		var body struct {
			Apps []appRow `json:"apps"`
		}
		if err := json.Unmarshal(buf.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if len(body.Apps) != 1 || body.Apps[0].ID != "opencode" || body.Apps[0].Installed {
			t.Fatalf("unexpected payload: %s", buf.String())
		}
	})

	t.Run("installed-only filter", func(t *testing.T) {
		var buf bytes.Buffer
		if err := runAppsTo(&buf, []string{"--installed"}); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(buf.String(), "0/0 installed") {
			t.Fatalf("expected empty installed set:\n%s", buf.String())
		}
	})
}
