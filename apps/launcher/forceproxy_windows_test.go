//go:build windows

package main

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestChromiumProxyArgs(t *testing.T) {
	got := chromiumProxyArgs("127.0.0.1:8081")
	if got[0] != "--proxy-server=http://127.0.0.1:8081" {
		t.Fatalf("got %v", got)
	}
	if got[1] != "--proxy-bypass-list=<local>" {
		t.Fatalf("got %v", got)
	}
}

func TestParseAppxManifestExe(t *testing.T) {
	xml := `<?xml version="1.0" encoding="utf-8"?>`
	xml += `<Package xmlns="http://schemas.microsoft.com/appx/2010/manifest">`
	xml += `<Applications><Application Id="App" Executable="app\ChatGPT.exe" /></Applications></Package>`
	rel, err := parseAppxManifestExe([]byte(xml))
	if err != nil || rel != `app\ChatGPT.exe` {
		t.Fatalf("rel=%q err=%v", rel, err)
	}
}

func TestMitmAddressFallback(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	if got := mitmAddress(); got != defaultMITMProxy {
		t.Fatalf("got %q want %q", got, defaultMITMProxy)
	}
}

// TestOpenAppChromium_UnresolvableSignalsNoPerProcessProxy pins that an app we
// cannot launch with per-process switches (here: a path that is neither AUMID
// nor an existing file) signals the caller instead of launching unproxied.
func TestOpenAppChromium_UnresolvableSignalsNoPerProcessProxy(t *testing.T) {
	app := catalogApp{Path: filepath.Join(t.TempDir(), "missing.exe")}
	if err := openAppChromium(app); !errors.Is(err, errNoPerProcessProxy) {
		t.Fatalf("got %v want errNoPerProcessProxy", err)
	}
}
