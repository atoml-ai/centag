package engine

import (
	"testing"

	"centag/apps/wrap/internal/snapshot"
)

// default (CA-only) enable records the CA but NO proxy restore point, so a
// later disable keeps the OS proxy exactly as the user left it (§3.4 Step 1).
func TestDisable_CAOnly_DoesNotRestoreProxy(t *testing.T) {
	testHome(t)
	snap := &snapshot.Snapshot{
		ClientMode:     "local",
		ProxyTakenOver: false,
		CA:             snapshot.CAState{FingerprintSHA256: "fp-ca-only", InstalledByUs: true},
	}
	if err := snapshot.Save(snap); err != nil {
		t.Fatal(err)
	}

	// The user has since changed the OS proxy to their own external setting.
	mos := &mockOS{
		supported: true,
		proxy:     snapshot.ProxyState{Mode: "manual", HTTP: "127.0.0.1:12000", HTTPS: "127.0.0.1:12000"},
	}
	e := &Engine{OS: mos}
	if err := e.Disable(); err != nil {
		t.Fatalf("disable: %v", err)
	}
	// Proxy untouched (still the user's), CA removed by fingerprint.
	if mos.proxy.Mode != "manual" || mos.proxy.HTTP != "127.0.0.1:12000" {
		t.Fatalf("CA-only disable must not restore proxy, got %+v", mos.proxy)
	}
	if mos.uninstalled != "fp-ca-only" {
		t.Fatalf("CA must be removed by fingerprint, got %q", mos.uninstalled)
	}
}

// A --system-proxy enable records the takeover and disable restores it.
func TestDisable_ProxyTakenOver_Restores(t *testing.T) {
	testHome(t)
	snap := &snapshot.Snapshot{
		ClientMode:     "local",
		ProxyTakenOver: true,
		Proxy:          snapshot.ProxyState{Mode: "manual", HTTP: "10.0.0.9:3128", HTTPS: "10.0.0.9:3128"},
		CA:             snapshot.CAState{FingerprintSHA256: "fp-takeover", InstalledByUs: true},
	}
	if err := snapshot.Save(snap); err != nil {
		t.Fatal(err)
	}
	mos := &mockOS{supported: true, proxy: snapshot.ProxyState{Mode: "pac", PACURL: "http://centag/pac"}}
	e := &Engine{OS: mos}
	if err := e.Disable(); err != nil {
		t.Fatalf("disable: %v", err)
	}
	if mos.proxy.Mode != "manual" || mos.proxy.HTTP != "10.0.0.9:3128" {
		t.Fatalf("expected proxy restore to 10.0.0.9:3128, got %+v", mos.proxy)
	}
	if mos.uninstalled != "fp-takeover" {
		t.Fatalf("expected CA removed, got %q", mos.uninstalled)
	}
}

// Legacy snapshots (written before ProxyTakenOver existed) with a non-empty
// Proxy must still be restored for backward compatibility.
func TestDisable_LegacySnapshot_FallsBackToProxyMode(t *testing.T) {
	testHome(t)
	snap := &snapshot.Snapshot{
		ClientMode: "local",
		Proxy:      snapshot.ProxyState{Mode: "manual", HTTP: "10.1.1.1:8080", HTTPS: "10.1.1.1:8080"},
		CA:         snapshot.CAState{FingerprintSHA256: "fp-legacy", InstalledByUs: true},
	}
	if err := snapshot.Save(snap); err != nil {
		t.Fatal(err)
	}
	mos := &mockOS{supported: true, proxy: snapshot.ProxyState{Mode: "pac", PACURL: "http://centag/pac"}}
	e := &Engine{OS: mos}
	if err := e.Disable(); err != nil {
		t.Fatalf("disable: %v", err)
	}
	if mos.proxy.Mode != "manual" || mos.proxy.HTTP != "10.1.1.1:8080" {
		t.Fatalf("legacy snapshot proxy not restored, got %+v", mos.proxy)
	}
}

// DisableWithOptions{CAOnly} removes the CA but leaves the proxy untouched.
func TestDisableWithOptions_CAOnly(t *testing.T) {
	testHome(t)
	snap := &snapshot.Snapshot{
		ClientMode:     "local",
		ProxyTakenOver: true,
		Proxy:          snapshot.ProxyState{Mode: "off"},
		CA:             snapshot.CAState{FingerprintSHA256: "fp-ca", InstalledByUs: true},
	}
	if err := snapshot.Save(snap); err != nil {
		t.Fatal(err)
	}
	mos := &mockOS{supported: true, proxy: snapshot.ProxyState{Mode: "pac", PACURL: "http://centag/pac"}}
	e := &Engine{OS: mos}
	if err := e.DisableWithOptions(DisableOptions{CAOnly: true}); err != nil {
		t.Fatalf("disable ca-only: %v", err)
	}
	if mos.uninstalled != "fp-ca" {
		t.Fatalf("CA must be removed, got %q", mos.uninstalled)
	}
	if mos.proxy.Mode != "pac" {
		t.Fatalf("proxy must be untouched with --ca-only, got %+v", mos.proxy)
	}
}

// DisableWithOptions{ProxyOnly} restores the proxy but keeps the CA installed.
func TestDisableWithOptions_ProxyOnly(t *testing.T) {
	testHome(t)
	snap := &snapshot.Snapshot{
		ClientMode:     "local",
		ProxyTakenOver: true,
		Proxy:          snapshot.ProxyState{Mode: "manual", HTTP: "10.2.2.2:8080", HTTPS: "10.2.2.2:8080"},
		CA:             snapshot.CAState{FingerprintSHA256: "fp-keep", InstalledByUs: true},
	}
	if err := snapshot.Save(snap); err != nil {
		t.Fatal(err)
	}
	mos := &mockOS{supported: true, proxy: snapshot.ProxyState{Mode: "pac", PACURL: "http://centag/pac"}}
	e := &Engine{OS: mos}
	if err := e.DisableWithOptions(DisableOptions{ProxyOnly: true}); err != nil {
		t.Fatalf("disable proxy-only: %v", err)
	}
	if mos.proxy.Mode != "manual" || mos.proxy.HTTP != "10.2.2.2:8080" {
		t.Fatalf("proxy must be restored with --proxy-only, got %+v", mos.proxy)
	}
	if mos.uninstalled != "" {
		t.Fatalf("CA must be kept with --proxy-only, got uninstalled=%q", mos.uninstalled)
	}
	if !snapshot.Exists() {
		t.Fatal("expected snapshot retained after proxy-only disable")
	}
}

// A CA-only enable must not persist a proxy restore point even though the
// system already had an external manual proxy (regression guard for P1-2).
func TestEnable_SystemProxyFalse_RecordsNoProxy(t *testing.T) {
	testHome(t)
	snap := &snapshot.Snapshot{
		ClientMode:     "local",
		ProxyTakenOver: false,
		CA:             snapshot.CAState{FingerprintSHA256: "fp", InstalledByUs: true},
	}
	if err := snapshot.Save(snap); err != nil {
		t.Fatal(err)
	}
	loaded, err := snapshot.Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ProxyTakenOver {
		t.Fatal("ProxyTakenOver must be false for CA-only snapshot")
	}
	if loaded.Proxy.Mode != "" {
		t.Fatalf("CA-only snapshot must not carry a proxy restore point, got %+v", loaded.Proxy)
	}
}
