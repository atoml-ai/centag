package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPrepareWrapApp(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/api/v1/wrap/apps/opencode/prepare") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"model":"centag/transparent","warnings":["未取到出口 Key"]}`))
	}))
	defer srv.Close()

	res, err := prepareWrapApp(srv.URL, "opencode", true)
	if err != nil {
		t.Fatal(err)
	}
	if res["model"] != "centag/transparent" {
		t.Fatalf("model=%v", res["model"])
	}
	if w := warningsText(res); w != "未取到出口 Key" {
		t.Fatalf("warnings=%q", w)
	}
}

func TestPrepareWrapApp_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"unknown app id"}`))
	}))
	defer srv.Close()
	if _, err := prepareWrapApp(srv.URL, "nope", false); err == nil {
		t.Fatal("expected error")
	}
}

func TestFormatDoctor(t *testing.T) {
	ready := map[string]any{
		"ok": true,
		"checks": []any{
			map[string]any{"ok": true, "message": "sidecar 运行中"},
		},
	}
	if got := formatDoctor(ready); got != "全部就绪" {
		t.Fatalf("got %q", got)
	}

	bad := map[string]any{
		"ok": false,
		"checks": []any{
			map[string]any{"ok": true, "message": "sidecar 运行中"},
			map[string]any{"ok": false, "message": "MITM 代理未启用", "action": "Web 开启 MITM"},
		},
	}
	got := formatDoctor(bad)
	if !strings.Contains(got, "MITM 代理未启用") || !strings.Contains(got, "Web 开启 MITM") {
		t.Fatalf("got %q", got)
	}
}
