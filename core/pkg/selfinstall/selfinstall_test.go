package selfinstall

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestEditionFromName(t *testing.T) {
	cases := map[string]string{
		"personal": "personal",
		"Team":     "team",
		" desktop": "desktop",
		"v1.2":     "",
		"":         "",
		"../etc":   "",
	}
	for in, want := range cases {
		if got := editionFromName(in); got != want {
			t.Errorf("editionFromName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDetectEditionPrefersEnv(t *testing.T) {
	t.Setenv("CENTAG_EDITION", "team")
	if got := detectEdition(); got != "team" {
		t.Errorf("detectEdition() = %q, want team", got)
	}
	t.Setenv("CENTAG_EDITION", "../../evil")
	if got := detectEdition(); got != "personal" {
		t.Errorf("detectEdition() with unsafe env = %q, want personal fallback", got)
	}
}

func TestRenderShimsMatchInstallShContract(t *testing.T) {
	cmd := renderWindowsShim("personal")
	for _, want := range []string{
		"@echo off",
		"set ROOT=%~dp0..",
		`set LIB=%ROOT%\lib\personal`,
		`set BIN=%LIB%\centag-personal.exe`,
		`if "%STATIC_PATH%"=="" set STATIC_PATH=%LIB%\static`,
		`if "%PROJECT_ROOT%"=="" set PROJECT_ROOT=%LIB%`,
		`if exist "%LIB%\config\profiles\personal\initdata"`,
		`"%BIN%" %*`,
	} {
		if !strings.Contains(cmd, want) {
			t.Errorf("windows shim missing %q\nshim:\n%s", want, cmd)
		}
	}

	sh := renderUnixShim("personal")
	for _, want := range []string{
		"#!/usr/bin/env bash",
		`EDITION="personal"`,
		`LIB="$ROOT/lib/$EDITION"`,
		`BIN="$LIB/centag-${EDITION}"`,
		`export STATIC_PATH="${STATIC_PATH:-$LIB/static}"`,
		`exec "$BIN" "$@"`,
	} {
		if !strings.Contains(sh, want) {
			t.Errorf("unix shim missing %q\nshim:\n%s", want, sh)
		}
	}
}

func withFakeHome(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)
	t.Setenv("SHELL", "bash")
	return tmp
}

func TestWriteEnvFiles(t *testing.T) {
	root := t.TempDir()
	binDir := filepath.Join(root, "bin")
	L := layout{root: root, binDir: binDir}
	if err := writeEnvFiles(L); err != nil {
		t.Fatal(err)
	}
	envData, err := os.ReadFile(filepath.Join(root, "env"))
	if err != nil {
		t.Fatal(err)
	}
	env := string(envData)
	for _, want := range []string{
		"case \":$PATH:\" in",
		`*) export PATH="` + toSlash(binDir) + `:$PATH" ;;`,
		"hash -r",
	} {
		if !strings.Contains(env, want) {
			t.Errorf("env helper missing %q\ncontent:\n%s", want, env)
		}
	}
	fishData, err := os.ReadFile(filepath.Join(root, "env.fish"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(fishData), "fish_add_path -g "+toSlash(binDir)) {
		t.Errorf("env.fish missing fish_add_path line:\n%s", fishData)
	}
}

func TestFilterRcLines(t *testing.T) {
	root := "/home/u/.centag"
	binDir := filepath.Join(root, "bin")
	lines := []string{
		"# my rc",
		"",
		"# centag (added by install.sh)",
		"[ -f \"" + root + "/env\" ] && . \"" + root + "/env\"",
		"",
		"export NVM_DIR=\"$HOME/.nvm\"",
		"export PATH=\"" + binDir + ":$PATH\"",
		"fish_add_path -g " + binDir,
		"alias ll='ls -l'",
	}
	out, removed := filterRcLines(lines, root, binDir)
	// marker + source line + trailing blank + legacy export + fish_add_path
	if removed != 5 {
		t.Errorf("removed = %d, want 5\nout: %q", removed, out)
	}
	joined := strings.Join(out, "\n")
	for _, want := range []string{"# my rc", "export NVM_DIR", "alias ll='ls -l'"} {
		if !strings.Contains(joined, want) {
			t.Errorf("kept lines lost %q\nout: %q", want, out)
		}
	}
	if strings.Contains(joined, binDir) || strings.Contains(joined, "# centag") {
		t.Errorf("centag entries survived removal:\n%s", joined)
	}

	// Non-centag lines that merely mention a similar path must survive.
	kept, removed2 := filterRcLines([]string{
		"export PATH=\"/opt/centag/bin:$PATH\"",
		"export PATH=\"" + binDir + "/extra:$PATH\"", // contains binDir but no export-keyword match? it has export PATH → removed
	}, root, binDir)
	if removed2 != 1 {
		t.Errorf("removed2 = %d, want 1\nout: %q", removed2, kept)
	}
	if !strings.Contains(strings.Join(kept, "\n"), "/opt/centag/bin") {
		t.Errorf("unrelated PATH entry was removed: %q", kept)
	}
}

func TestParseOptions(t *testing.T) {
	opts, help, err := ParseOptions([]string{"--prefix", "/tmp/c", "--bin-dir", "/tmp/c/bin", "--no-modify-path"})
	if err != nil || help {
		t.Fatalf("parse: err=%v help=%v", err, help)
	}
	if opts.Root != "/tmp/c" || opts.BinDir != "/tmp/c/bin" || !opts.NoModifyPath {
		t.Errorf("opts = %+v", opts)
	}
	if _, help, err := ParseOptions([]string{"-h"}); err != nil || !help {
		t.Errorf("help: err=%v help=%v", err, help)
	}
	if _, _, err := ParseOptions([]string{"--bogus"}); err == nil {
		t.Error("unknown flag should error")
	}
	if _, _, err := ParseOptions([]string{"positional"}); err == nil {
		t.Error("positional argument should error")
	}
}

// TestRunInitAndUninstallFlow exercises the full init/uninstall round-trip in
// an isolated prefix (and fake HOME for rc handling). The registry / rc PATH
// persistence is exercised on unix; on windows the test runs with
// --no-modify-path so the developer's real registry is never touched.
func TestRunInitAndUninstallFlow(t *testing.T) {
	fakeHome := withFakeHome(t)
	root := filepath.Join(t.TempDir(), "centag-root")
	binDir := filepath.Join(root, "bin")

	var stdout, stderr bytes.Buffer
	opts := Options{Root: root, BinDir: binDir, Stdout: &stdout, Stderr: &stderr}
	if runtime.GOOS == "windows" {
		opts.NoModifyPath = true
	}

	if err := RunInit(opts); err != nil {
		t.Fatalf("RunInit: %v\nstdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
	}

	// Shim written with the install.sh contract.
	shim := shimPath(layout{binDir: binDir})
	data, err := os.ReadFile(shim)
	if err != nil {
		t.Fatalf("shim missing: %v", err)
	}
	if !strings.Contains(string(data), "EDITION=personal") && !strings.Contains(string(data), `EDITION="personal"`) {
		t.Errorf("shim missing edition line:\n%s", data)
	}

	// env helpers written.
	if _, err := os.Stat(filepath.Join(root, "env")); err != nil {
		t.Fatalf("env helper missing: %v", err)
	}

	if runtime.GOOS != "windows" {
		rcData, err := os.ReadFile(filepath.Join(fakeHome, ".bashrc"))
		if err != nil {
			t.Fatalf("rc file not created: %v", err)
		}
		if !strings.Contains(string(rcData), "# centag (added by centag install)") {
			t.Errorf("rc block missing:\n%s", rcData)
		}
	}

	// Re-run must stay idempotent (no duplicate rc blocks / no errors).
	stdout.Reset()
	stderr.Reset()
	if err := RunInit(opts); err != nil {
		t.Fatalf("RunInit (idempotent): %v", err)
	}
	if runtime.GOOS != "windows" {
		rcData, _ := os.ReadFile(filepath.Join(fakeHome, ".bashrc"))
		if got := strings.Count(string(rcData), "# centag (added by centag install)"); got != 1 {
			t.Errorf("rc block count after re-init = %d, want 1:\n%s", got, rcData)
		}
	}

	// Uninstall removes shim, env helpers and PATH block.
	stdout.Reset()
	stderr.Reset()
	if err := RunUninstall(opts); err != nil {
		t.Fatalf("RunUninstall: %v", err)
	}
	if _, err := os.Stat(shim); !os.IsNotExist(err) {
		t.Errorf("shim still present after uninstall (err=%v)", err)
	}
	if _, err := os.Stat(filepath.Join(root, "env")); !os.IsNotExist(err) {
		t.Errorf("env helper still present after uninstall")
	}
	if runtime.GOOS != "windows" {
		rcData, _ := os.ReadFile(filepath.Join(fakeHome, ".bashrc"))
		if strings.Contains(string(rcData), "centag") {
			t.Errorf("rc still references centag after uninstall:\n%s", rcData)
		}
	}
}
