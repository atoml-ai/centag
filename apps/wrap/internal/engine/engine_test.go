package engine

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"centag/apps/wrap/internal/snapshot"
)

type mockOS struct {
	proxy        snapshot.ProxyState
	writePACURL  string
	installedPEM []byte
	fp           string
	uninstalled  string
	writeErr     error
	installErr   error
	supported    bool
	detail       string
}

func (m *mockOS) Supported() (bool, string) {
	if m.detail == "" {
		m.detail = "mock"
	}
	return m.supported, m.detail
}

func (m *mockOS) ReadProxy() (snapshot.ProxyState, error) {
	return m.proxy, nil
}

func (m *mockOS) WritePAC(pacURL string) error {
	if m.writeErr != nil {
		return m.writeErr
	}
	m.writePACURL = pacURL
	return nil
}

func (m *mockOS) RestoreProxy(state snapshot.ProxyState) error {
	m.proxy = state
	return nil
}

func (m *mockOS) InstallCA(certPEM []byte) (string, error) {
	if m.installErr != nil {
		return "", m.installErr
	}
	m.installedPEM = certPEM
	if m.fp == "" {
		m.fp = "deadbeefcafe"
	}
	return m.fp, nil
}

func (m *mockOS) UninstallCA(fingerprint string) error {
	m.uninstalled = fingerprint
	return nil
}

func testHome(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("LOCALAPPDATA", dir)
	_ = os.Remove(filepath.Join(dir, ".centag", "proxy-snapshot.json"))
}

func mustCA(t *testing.T) []byte {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "Centag CA"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		IsCA:         true,
		KeyUsage:     x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

func TestEnableRemote_SuccessAndDisableDoesNotNeedServer(t *testing.T) {
	testHome(t)
	ca := mustCA(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/proxy/pac":
			_, _ = w.Write([]byte(`return "PROXY 192.168.1.50:8081";`))
		case "/api/v1/proxy/ca.crt":
			_, _ = w.Write(ca)
		case "/api/v1/proxy/setup/status":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"mode":"lan","allow_lan_clients":true,"advertise_host":"192.168.1.50",
				"pac_url":"http://192.168.1.50:20060/api/v1/proxy/pac",
				"mitm_proxy":"192.168.1.50:8081","listen_is_loopback":false
			}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	mos := &mockOS{
		supported: true,
		proxy:     snapshot.ProxyState{Mode: "pac", PACURL: "http://old.example/pac"},
		fp:        "abc123fingerprint",
	}
	e := &Engine{OS: mos}
	if err := e.Enable(srv.URL); err != nil {
		t.Fatalf("enable: %v", err)
	}
	if mos.writePACURL != "http://192.168.1.50:20060/api/v1/proxy/pac" {
		t.Fatalf("WritePAC=%q", mos.writePACURL)
	}
	if !snapshot.Exists() {
		t.Fatal("expected snapshot")
	}
	snap, _ := snapshot.Load()
	if snap.ClientMode != "remote" {
		t.Fatalf("mode=%s", snap.ClientMode)
	}

	// Stop server — disable must still restore locally
	srv.Close()
	if err := e.Disable(); err != nil {
		t.Fatalf("disable: %v", err)
	}
	if mos.proxy.PACURL != "http://old.example/pac" {
		t.Fatalf("restored pac=%q", mos.proxy.PACURL)
	}
	if mos.uninstalled != "abc123fingerprint" {
		t.Fatalf("uninstall fp=%q", mos.uninstalled)
	}
	if snapshot.Exists() {
		t.Fatal("snapshot should be removed")
	}
}

func TestEnableRemote_RejectsLoopbackPAC(t *testing.T) {
	testHome(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/proxy/pac":
			_, _ = w.Write([]byte(`return "PROXY 127.0.0.1:8081";`))
		case "/api/v1/proxy/setup/status":
			http.Error(w, "unauthorized", http.StatusUnauthorized)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	e := &Engine{OS: &mockOS{supported: true}}
	err := e.Enable(srv.URL)
	if err == nil || !strings.Contains(err.Error(), "127.0.0.1") {
		t.Fatalf("expected loopback PAC error, got %v", err)
	}
	if snapshot.Exists() {
		t.Fatal("snapshot should be rolled back")
	}
}

func TestEnableRemote_RejectsWhenLANDisabled(t *testing.T) {
	testHome(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/proxy/pac":
			_, _ = w.Write([]byte(`PROXY 192.168.1.1:8081`))
		case "/api/v1/proxy/setup/status":
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `{"allow_lan_clients":false,"advertise_host":"192.168.1.1","pac_url":"http://x/pac"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	e := &Engine{OS: &mockOS{supported: true}}
	if err := e.Enable(srv.URL); err == nil || !strings.Contains(err.Error(), "allow_lan_clients") {
		t.Fatalf("got %v", err)
	}
}

func TestEnable_AlreadyEnabled(t *testing.T) {
	testHome(t)
	_ = snapshot.Save(&snapshot.Snapshot{ClientMode: "local"})
	e := &Engine{OS: &mockOS{supported: true}}
	if err := e.Enable(""); err == nil || !strings.Contains(err.Error(), "already enabled") {
		t.Fatalf("got %v", err)
	}
}

func TestEnableLocal_Success(t *testing.T) {
	testHome(t)
	ca := mustCA(t)
	var base string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/proxy/ca.crt":
			_, _ = w.Write(ca)
		case "/api/v1/proxy/pac":
			_, _ = w.Write([]byte(`PROXY 127.0.0.1:8081`))
		case "/api/v1/proxy/setup/status":
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{"allow_lan_clients":false,"pac_url":"%s/api/v1/proxy/pac","mitm_proxy":"127.0.0.1:8081"}`, base)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	base = srv.URL
	t.Setenv("CENTAG_API_BASE", srv.URL)

	mos := &mockOS{supported: true, fp: "localfp", proxy: snapshot.ProxyState{Mode: "off"}}
	e := &Engine{OS: mos}
	if err := e.Enable(""); err != nil {
		t.Fatal(err)
	}
	if mos.writePACURL != base+"/api/v1/proxy/pac" {
		t.Fatalf("pac=%q", mos.writePACURL)
	}
	if err := e.Disable(); err != nil {
		t.Fatal(err)
	}
}

func TestEnableLocal_RollbackOnWritePACFail(t *testing.T) {
	testHome(t)
	ca := mustCA(t)
	var base string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/proxy/ca.crt":
			_, _ = w.Write(ca)
		case "/api/v1/proxy/setup/status":
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{"allow_lan_clients":false,"pac_url":"%s/api/v1/proxy/pac","mitm_proxy":"127.0.0.1:8081"}`, base)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	base = srv.URL
	t.Setenv("CENTAG_API_BASE", srv.URL)
	mos := &mockOS{supported: true, fp: "f", writeErr: fmt.Errorf("denied")}
	e := &Engine{OS: mos}
	if err := e.Enable(""); err == nil || !strings.Contains(err.Error(), "write PAC") {
		t.Fatalf("got %v", err)
	}
	if snapshot.Exists() {
		t.Fatal("expected rollback")
	}
}

func TestEnableLocal_RollbackOnCAFail(t *testing.T) {
	testHome(t)
	var base string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/proxy/ca.crt":
			_, _ = w.Write(mustCA(t))
		case "/api/v1/proxy/pac":
			_, _ = w.Write([]byte(`PROXY 127.0.0.1:8081`))
		case "/api/v1/proxy/setup/status":
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{"allow_lan_clients":false,"pac_url":"%s/api/v1/proxy/pac","mitm_proxy":"127.0.0.1:8081"}`, base)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	base = srv.URL
	t.Setenv("CENTAG_API_BASE", srv.URL)

	mos := &mockOS{supported: true, installErr: fmt.Errorf("no sudo"), proxy: snapshot.ProxyState{Mode: "off"}}
	e := &Engine{OS: mos}
	if err := e.Enable(""); err == nil || !strings.Contains(err.Error(), "install CA") {
		t.Fatalf("got %v", err)
	}
	if snapshot.Exists() {
		t.Fatal("expected rollback remove snapshot")
	}
}

func TestDoctor_Pass(t *testing.T) {
	testHome(t)
	ca := mustCA(t)
	var base string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/proxy/pac":
			_, _ = w.Write([]byte(`PROXY 10.0.0.1:8081`))
		case "/api/v1/proxy/ca.crt":
			_, _ = w.Write(ca)
		case "/api/v1/proxy/setup/status":
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{"mode":"lan","allow_lan_clients":true,"advertise_host":"10.0.0.1","pac_url":"%s/api/v1/proxy/pac","listen_is_loopback":false}`, base)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	base = srv.URL

	e := &Engine{OS: &mockOS{supported: true, detail: "mock-os"}}
	if err := e.Doctor(srv.URL); err != nil {
		t.Fatal(err)
	}
}

func TestStatus_WithSnapshot(t *testing.T) {
	testHome(t)
	_ = snapshot.Save(&snapshot.Snapshot{
		ClientMode: "remote",
		Centag:     snapshot.CentagRef{APIBase: "http://team:20060"},
		CA:         snapshot.CAState{FingerprintSHA256: "0123456789abcdef"},
	})
	e := &Engine{OS: &mockOS{supported: true, proxy: snapshot.ProxyState{Mode: "pac", PACURL: "http://x"}}}
	if err := e.Status(); err != nil {
		t.Fatal(err)
	}
}

func TestEnableRemote_RollbackOnWritePACFail(t *testing.T) {
	testHome(t)
	ca := mustCA(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/proxy/pac":
			_, _ = w.Write([]byte(`PROXY 192.168.1.50:8081`))
		case "/api/v1/proxy/ca.crt":
			_, _ = w.Write(ca)
		case "/api/v1/proxy/setup/status":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"allow_lan_clients":true,"advertise_host":"192.168.1.50","pac_url":"http://192.168.1.50/pac","mitm_proxy":"192.168.1.50:8081"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	mos := &mockOS{supported: true, writeErr: fmt.Errorf("permission denied"), fp: "x"}
	e := &Engine{OS: mos}
	if err := e.Enable(srv.URL); err == nil {
		t.Fatal("expected write PAC error")
	}
	if snapshot.Exists() {
		t.Fatal("snapshot should be rolled back")
	}
}

func TestDisable_NoSnapshot(t *testing.T) {
	testHome(t)
	e := &Engine{OS: &mockOS{supported: true}}
	if err := e.Disable(); err == nil {
		t.Fatal("expected error")
	}
}

func TestDoctor_UsesSnapshotAPIBase(t *testing.T) {
	testHome(t)
	ca := mustCA(t)
	var base string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/proxy/pac":
			_, _ = w.Write([]byte(`PROXY 10.0.0.1:8081`))
		case "/api/v1/proxy/ca.crt":
			_, _ = w.Write(ca)
		case "/api/v1/proxy/setup/status":
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{"mode":"lan","allow_lan_clients":true,"advertise_host":"10.0.0.1","pac_url":"%s/pac","listen_is_loopback":false}`, base)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	base = srv.URL
	_ = snapshot.Save(&snapshot.Snapshot{ClientMode: "remote", Centag: snapshot.CentagRef{APIBase: srv.URL}})
	e := &Engine{OS: &mockOS{supported: true}}
	if err := e.Doctor(""); err != nil {
		t.Fatal(err)
	}
}
