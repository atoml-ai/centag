package configsync

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func snapshotJSON(body string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}
}

const goodSnapshot = `{"schema":1,"generated_at":"2026-08-30T00:00:00Z",
 "config":[{"edition":"all","config_key":"table.model_price","value":{"provider":"feishu"},"enabled":true}],
 "prices":[{"base_url":"https://api.ppinfra.com/v3/openai","provider_name":"PPIO","currency":"USD","enabled":true,
   "models":[{"model":"deepseek-v3.2","name":"DeepSeek V3.2","input_price_per_m":0.1389,"output_price_per_m":0.1389,"cost_multiplier":1}]}]}`

// ---------- E. snapshot Provider（TC-SNP-001~006） ----------

func TestSnapshotProvider(t *testing.T) {
	ctx := context.Background()

	t.Run("TC-SNP-001_主源解析", func(t *testing.T) {
		ts := httptest.NewServer(snapshotJSON(goodSnapshot))
		defer ts.Close()
		p := NewSnapshotProvider([]string{ts.URL})
		rows, err := p.FetchConfig(ctx, Query{})
		if err != nil || len(rows) != 1 || rows[0].Key != "table.model_price" {
			t.Fatalf("rows=%v err=%v", rows, err)
		}
		prices, err := p.FetchModelPrices(ctx)
		if err != nil || len(prices) != 1 || prices[0].Models[0].Model != "deepseek-v3.2" {
			t.Fatalf("prices=%v err=%v", prices, err)
		}
	})

	t.Run("TC-SNP-002_多源回落", func(t *testing.T) {
		bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer bad.Close()
		good := httptest.NewServer(snapshotJSON(goodSnapshot))
		defer good.Close()
		p := NewSnapshotProvider([]string{bad.URL, good.URL})
		rows, err := p.FetchConfig(ctx, Query{})
		if err != nil || len(rows) != 1 {
			t.Fatalf("fallback failed: rows=%d err=%v", len(rows), err)
		}
	})

	t.Run("TC-SNP-003_全源失败沿用报错", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer ts.Close()
		p := NewSnapshotProvider([]string{ts.URL, ts.URL})
		if _, err := p.FetchConfig(ctx, Query{}); err == nil || !strings.Contains(err.Error(), "all sources failed") {
			t.Fatalf("want all-sources-failed error, got %v", err)
		}
	})

	t.Run("TC-SNP-004_超尺寸拒收", func(t *testing.T) {
		old := maxSnapshotBody
		maxSnapshotBody = 1024
		defer func() { maxSnapshotBody = old }()
		ts := httptest.NewServer(snapshotJSON(`{"schema":1,"config":[],"junk":"` + strings.Repeat("x", 4096) + `"}`))
		defer ts.Close()
		p := NewSnapshotProvider([]string{ts.URL})
		if _, err := p.FetchConfig(ctx, Query{}); err == nil || !strings.Contains(err.Error(), "size cap") {
			t.Fatalf("oversize must be rejected, got %v", err)
		}
	})

	t.Run("TC-SNP-005_schema不符拒收", func(t *testing.T) {
		ts := httptest.NewServer(snapshotJSON(`{"schema":999,"config":[]}`))
		defer ts.Close()
		p := NewSnapshotProvider([]string{ts.URL})
		if _, err := p.FetchConfig(ctx, Query{}); err == nil || !strings.Contains(err.Error(), "schema") {
			t.Fatalf("schema mismatch must be rejected, got %v", err)
		}
	})

	t.Run("TC-SNP-006_env覆盖URL", func(t *testing.T) {
		t.Setenv("CENTAG_CONFIGSYNC_SNAPSHOT_URL", "https://mirror.example.com/snap.json https://backup.example.com/snap.json")
		urls := ResolveSnapshotURLs([]string{"https://default.example.com/snap.json"})
		if len(urls) != 2 || urls[0] != "https://mirror.example.com/snap.json" {
			t.Fatalf("env override failed: %v", urls)
		}
		if got := ResolveSnapshotURLs(nil); len(got) != 2 {
			t.Fatalf("multi-source parse failed: %v", got)
		}
	})
}
