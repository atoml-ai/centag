//go:build darwin

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestResolveMacAppBundle(t *testing.T) {
	root := t.TempDir()
	appDir := filepath.Join(root, "WorkBuddy.app")
	macosDir := filepath.Join(appDir, "Contents", "MacOS")
	if err := os.MkdirAll(macosDir, 0o755); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(macosDir, "WorkBuddy")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	plist := `<?xml version="1.0" encoding="UTF-8"?>
<plist version="1.0"><dict>
<key>CFBundleExecutable</key><string>WorkBuddy</string>
</dict></plist>`
	if err := os.WriteFile(filepath.Join(appDir, "Contents", "Info.plist"), []byte(plist), 0o644); err != nil {
		t.Fatal(err)
	}

	got, ok := resolveMacAppBundle(appDir)
	if !ok {
		t.Fatalf("expected resolution for %s", appDir)
	}
	if got != exe {
		t.Fatalf("got %q want %q", got, exe)
	}

	if got, ok := resolveMacAppBundle(appDir + "/"); !ok || got != exe {
		t.Fatalf("trailing slash: got %q ok=%v want %q", got, ok, exe)
	}
	if _, ok := resolveMacAppBundle("/usr/bin/opencode"); ok {
		t.Fatal("plain executable should not resolve")
	}
}

// electronPlist mirrors a real Electron app Info.plist: string values are
// interleaved with <true/>/<integer>/nested dicts, so naive index-based
// key/string pairing desyncs and picks the wrong CFBundleExecutable.
const electronPlist = `<?xml version="1.0" encoding="UTF-8"?>
<plist version="1.0">
<dict>
	<key>CFBundleDevelopmentRegion</key>
	<string>English</string>
	<key>CFBundleDisplayName</key>
	<string>WorkBuddy</string>
	<key>CFBundleExecutable</key>
	<string>Electron</string>
	<key>CFBundleIconFile</key>
	<string>electron.icns</string>
	<key>NSHighResolutionCapable</key>
	<true/>
	<key>CFBundleVersion</key>
	<string>8.6.1</string>
</dict>
</plist>
`

func TestResolveMacAppBundle_ElectronStylePlist(t *testing.T) {
	bundle := filepath.Join(t.TempDir(), "WorkBuddy.app")
	macosDir := filepath.Join(bundle, "Contents", "MacOS")
	if err := os.MkdirAll(macosDir, 0o755); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(macosDir, "Electron")
	if err := os.WriteFile(exe, []byte("\xff\xcf\xfa\xfe"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bundle, "Contents", "Info.plist"), []byte(electronPlist), 0o644); err != nil {
		t.Fatal(err)
	}

	got, ok := resolveMacAppBundle(bundle)
	if !ok {
		t.Fatal("expected resolution for Electron-style bundle")
	}
	if got != exe {
		t.Fatalf("got %q want %q", got, exe)
	}
}

func TestResolveMacAppBundle_RealWorkBuddy(t *testing.T) {
	const app = "/Applications/WorkBuddy.app"
	if _, err := os.Stat(app); err != nil {
		t.Skip("WorkBuddy.app not installed")
	}
	got, ok := resolveMacAppBundle(app)
	if !ok {
		t.Fatalf("failed to resolve %s", app)
	}
	if got != filepath.Join(app, "Contents/MacOS/Electron") {
		t.Fatalf("unexpected resolution: %s", got)
	}
}

func TestLaunchWrapped_LogsAndDetaches(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "marker.txt")
	cmdLine := "/bin/sh -c 'echo hello-detached > " + marker + "'"
	logPath, err := launchWrapped(cmdLine, "Test App v1.0")
	if err != nil {
		t.Fatal(err)
	}
	if logPath == "" {
		t.Fatal("expected a log path")
	}
	if !strings.HasPrefix(logPath, filepath.Join(centagInstallRoot(), "wrap", "runs")) {
		t.Fatalf("log outside runs dir: %s", logPath)
	}
	// Wait briefly for the detached process to write its marker.
	for i := 0; i < 50; i++ {
		if _, err := os.Stat(marker); err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("detached command did not run: %v", err)
	}
}

func TestLocateCentagCA(t *testing.T) {
	// Isolate the install root so the wrap-cache fallback can't interfere.
	t.Setenv("CENTAG_INSTALL_ROOT", t.TempDir())
	cfg := Config{DataDir: filepath.Join(t.TempDir(), "lib", "personal")}
	if got := locateCentagCA(cfg); got != "" {
		t.Fatalf("expected empty for missing CA, got %q", got)
	}
	dir := filepath.Join(cfg.DataDir, "certs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	ca := filepath.Join(dir, "ca.crt")
	if err := os.WriteFile(ca, []byte("dummy"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := locateCentagCA(cfg); got != ca {
		t.Fatalf("got %q want %q", got, ca)
	}
	// Fallback: wrap's downloaded CA (under the isolated install root) is used
	// when the lib install has none.
	if err := os.MkdirAll(filepath.Join(t.TempDir(), "unused"), 0o755); err != nil {
		t.Fatal(err)
	}
	wrapDir := filepath.Join(os.Getenv("CENTAG_INSTALL_ROOT"), "wrap")
	if err := os.MkdirAll(wrapDir, 0o755); err != nil {
		t.Fatal(err)
	}
	fallback := filepath.Join(wrapDir, "ca.crt")
	if err := os.WriteFile(fallback, []byte("dummy"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := locateCentagCA(Config{DataDir: filepath.Join(os.Getenv("CENTAG_INSTALL_ROOT"), "lib", "personal")}); got != fallback {
		t.Fatalf("wrap fallback: got %q want %q", got, fallback)
	}
}

func TestEnsureCATrusted_AlreadyTrustedNoOp(t *testing.T) {
	// On this machine the CA is already trusted (login keychain); ensure no
	// error and no dialog: the function must return silently.
	cfg := Config{DataDir: t.TempDir(), Headless: true}
	ensureCATrusted(cfg) // headless: must be a no-op, not panic

	cfg.Headless = false
	// locateCentagCA finds nothing in the temp dir → silent return without dialog.
	ensureCATrusted(cfg)
}

func TestEnsureCATrusted_RealCAPathTrusted(t *testing.T) {
	ca := locateCentagCA(Config{DataDir: filepath.Join(centagInstallRoot(), "lib", "personal")})
	if ca == "" {
		t.Skip("no CA on this machine")
	}
	if out, err := exec.Command("security", "verify-cert", "-c", ca, "-p", "ssl").CombinedOutput(); err != nil {
		t.Fatalf("CA should be trusted after login-keychain install: %v (%s)", err, out)
	}
}
