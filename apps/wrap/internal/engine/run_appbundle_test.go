package engine

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFakeAppBundle(t *testing.T, bundleName, cfBundleExec string) string {
	t.Helper()
	root := t.TempDir()
	appDir := filepath.Join(root, bundleName)
	macosDir := filepath.Join(appDir, "Contents", "MacOS")
	if err := os.MkdirAll(macosDir, 0o755); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(macosDir, cfBundleExec)
	if err := os.WriteFile(exe, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	plist := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleExecutable</key>
	<string>` + cfBundleExec + `</string>
</dict>
</plist>
`
	if err := os.WriteFile(filepath.Join(appDir, "Contents", "Info.plist"), []byte(plist), 0o644); err != nil {
		t.Fatal(err)
	}
	return appDir
}

func TestResolveMacAppBundle(t *testing.T) {
	// Resolves a .app directory to its Contents/MacOS binary via Info.plist.
	appDir := writeFakeAppBundle(t, "WorkBuddy.app", "WorkBuddy")
	got, ok := resolveMacAppBundle(appDir)
	if !ok {
		t.Fatalf("expected resolution for %s", appDir)
	}
	want := filepath.Join(appDir, "Contents", "MacOS", "WorkBuddy")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}

	// Trailing slash must be tolerated.
	if got, ok := resolveMacAppBundle(appDir + "/"); !ok || got != want {
		t.Fatalf("trailing slash: got %q ok=%v want %q", got, ok, want)
	}

	// Non-.app path is not resolved.
	if _, ok := resolveMacAppBundle("/usr/bin/opencode"); ok {
		t.Fatal("plain executable should not resolve")
	}

	// Missing .app suffix is not resolved.
	if _, ok := resolveMacAppBundle(filepath.Join(t.TempDir(), "notabundle")); ok {
		t.Fatal("directory without .app suffix should not resolve")
	}

	// Bundle whose binary is absent falls back to false.
	root := t.TempDir()
	badApp := filepath.Join(root, "Ghost.app")
	if err := os.MkdirAll(filepath.Join(badApp, "Contents", "MacOS"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, ok := resolveMacAppBundle(badApp); ok {
		t.Fatal("bundle without executable should not resolve")
	}
}

// electronPlist mirrors a real Electron app Info.plist: string values are
// interleaved with <true/>, <integer> and nested dicts, so naive index-based
// key/string pairing desyncs and picks the wrong CFBundleExecutable.
const electronPlist = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
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
	<key>CFBundleIdentifier</key>
	<string>com.electron.workbuddy</string>
	<key>LSMinimumSystemVersion</key>
	<string>10.13.0</string>
	<key>NSHighResolutionCapable</key>
	<true/>
	<key>CFBundleVersion</key>
	<string>8.6.1</string>
	<key>NSAppTransportSecurity</key>
	<dict>
		<key>NSAllowsArbitraryLoads</key>
		<true/>
	</dict>
	<key>ElectronAsarIntegrity</key>
	<dict>
		<key>Resources/app.asar</key>
		<dict>
			<key>algorithm</key>
			<string>SHA256</string>
			<key>digest</key>
			<string>deadbeef</string>
		</dict>
	</dict>
</dict>
</plist>
`

func TestResolveMacAppBundle_ElectronStylePlist(t *testing.T) {
	// CFBundleExecutable ("Electron") differs from the bundle name and is the
	// third <string> but the third <key>-with-string-value alignment fails on
	// real plists; the sequential parser must still find it.
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

	// Fallback: binary named differently than both bundle and CFBundleExecutable
	// must still be found by scanning Contents/MacOS.
	bundle2 := filepath.Join(t.TempDir(), "Other.app")
	macos2 := filepath.Join(bundle2, "Contents", "MacOS")
	if err := os.MkdirAll(macos2, 0o755); err != nil {
		t.Fatal(err)
	}
	exe2 := filepath.Join(macos2, "SomeBinary")
	if err := os.WriteFile(exe2, []byte("\xff\xcf\xfa\xfe"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bundle2, "Contents", "Info.plist"), []byte(electronPlist), 0o644); err != nil {
		t.Fatal(err)
	}
	got2, ok := resolveMacAppBundle(bundle2)
	if !ok || got2 != exe2 {
		t.Fatalf("Contents/MacOS scan fallback failed: got %q ok=%v", got2, ok)
	}
}

func TestResolveMacAppBundle_RealWorkBuddy(t *testing.T) {
	// Locks the fix against the actual app reported by the user. Skipped when
	// the app is not installed (CI machines).
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
