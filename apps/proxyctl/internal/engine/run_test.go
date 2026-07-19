package engine

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrepareProcessEnv_FromPAC(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	ca := mustCA(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/proxy/setup/status":
			w.WriteHeader(http.StatusUnauthorized)
		case "/api/v1/proxy/pac":
			_, _ = w.Write([]byte(`return "PROXY 192.168.1.50:8081";`))
		case "/api/v1/proxy/ca.crt":
			_, _ = w.Write(ca)
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	e := New()
	pe, err := e.PrepareProcessEnv(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if pe.MITM != "192.168.1.50:8081" {
		t.Fatalf("mitm=%q", pe.MITM)
	}
	if pe.Vars["HTTPS_PROXY"] != "http://192.168.1.50:8081" {
		t.Fatalf("HTTPS_PROXY=%q", pe.Vars["HTTPS_PROXY"])
	}
	if pe.Vars["NODE_EXTRA_CA_CERTS"] == "" {
		t.Fatal("missing NODE_EXTRA_CA_CERTS")
	}
	if _, err := os.Stat(pe.CAPath); err != nil {
		t.Fatal(err)
	}
	wantSuffix := filepath.Join(".centag", "proxyctl", "ca.crt")
	if !strings.HasSuffix(pe.CAPath, wantSuffix) {
		t.Fatalf("ca path=%q", pe.CAPath)
	}
}

func TestPrepareProcessEnv_RejectsLoopbackRemote(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	ca := mustCA(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/proxy/setup/status":
			_, _ = w.Write([]byte(`{"mitm_proxy":"127.0.0.1:8081","allow_lan_clients":false}`))
		case "/api/v1/proxy/ca.crt":
			_, _ = w.Write(ca)
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	e := New()
	if _, err := e.PrepareProcessEnv(srv.URL); err == nil {
		t.Fatal("expected error for loopback MITM with --server")
	}
}

func TestMergeEnv(t *testing.T) {
	base := []string{"PATH=/bin", "HTTPS_PROXY=old", "FOO=1"}
	out := mergeEnv(base, map[string]string{"HTTPS_PROXY": "http://x:8081", "NO_PROXY": "localhost"})
	joined := strings.Join(out, "\n")
	if strings.Contains(joined, "HTTPS_PROXY=old") {
		t.Fatal("old HTTPS_PROXY should be dropped")
	}
	if !strings.Contains(joined, "HTTPS_PROXY=http://x:8081") {
		t.Fatal("missing new HTTPS_PROXY")
	}
	if !strings.Contains(joined, "PATH=/bin") {
		t.Fatal("PATH should remain")
	}
}
