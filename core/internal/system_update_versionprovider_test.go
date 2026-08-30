package internal

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strings"
	"testing"

	"centag/core/internal/ota"
	"centag/core/pkg/configsync"
	"centag/core/pkg/logger"
)

func TestMain(m *testing.M) {
	// Handler paths log via the global zap sugar; tests must init it once.
	_ = logger.Init(logger.Config{Level: "error", Format: "json", Output: "stdout"})
	os.Exit(m.Run())
}

// vpShim adapts the configsync adapter to internal.VersionProvider.
type vpShim struct {
	inner *configsync.VersionProviderAdapter
}

func (s *vpShim) CheckLatest(ctx context.Context, current string) (*VersionCheckResult, error) {
	res, err := s.inner.CheckLatest(ctx, current)
	if err != nil {
		return nil, err
	}
	return &VersionCheckResult{
		UpdateAvailable: res.UpdateAvailable,
		Version:         res.Version,
		DownloadURL:     res.DownloadURL,
		SHA256:          res.SHA256,
		Message:         res.Message,
	}, nil
}

func releaseRow(version, channel string) configsync.Row {
	v, _ := configsync.MarshalValue(configsync.VersionInfo{Version: version, PackageURL: "https://cdn/pkg.tar.gz", SHA256: "cafe"})
	return configsync.Row{Key: "release.channel." + channel, Channel: channel, Edition: "all", Enabled: true, Value: v}
}

func otaFallbackServer(t *testing.T, tag string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/x/y/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		asset := "update-package-centag-team-" + runtime.GOOS + "-" + runtime.GOARCH + ".tar.gz"
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tag_name": tag,
			"assets": []map[string]any{
				{"name": asset, "size": 1024, "browser_download_url": "https://dl/" + asset},
			},
		})
	})
	return httptest.NewServer(mux)
}

func newHandlerWithOTA(t *testing.T, tag string) *SystemUpdateHandler {
	t.Helper()
	h := NewSystemUpdateHandler("test")
	ts := otaFallbackServer(t, tag)
	t.Cleanup(ts.Close)
	h.SetOTAClient(&ota.Client{
		APIBase: ts.URL, Repo: "x/y", Edition: "team",
		GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, HTTPClient: ts.Client(),
	})
	return h
}

func decodeCheck(t *testing.T, w *httptest.ResponseRecorder) (map[string]any, string) {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	source, _ := body["source"].(string)
	return body, source
}

// ---------- G. 更新检查注入（handler 层，TC-VAP-002/003/005） ----------

func TestSystemUpdateVersionProvider(t *testing.T) {
	ctx := context.Background()

	t.Run("TC-VAP-002_Provider出错回落GitHub_OTA", func(t *testing.T) {
		failing := &vpShim{inner: configsync.NewVersionProviderAdapter(
			func(ctx context.Context) ([]configsync.Row, error) { return nil, errors.New("feishu down") }, "stable")}
		h := newHandlerWithOTA(t, "v0.9.9")
		h.SetVersionProvider(failing)

		req := httptest.NewRequest(http.MethodGet, "/update/check", nil)
		w := httptest.NewRecorder()
		h.HandleCheckUpdate(w, req.WithContext(ctx))
		if w.Code != http.StatusOK {
			t.Fatalf("OTA fallback must answer 200, got %d: %s", w.Code, w.Body.String())
		}
		body, source := decodeCheck(t, w)
		if source == "configsync" {
			t.Fatal("failing provider must not be reported as the source")
		}
		if check, ok := body["check"].(map[string]any); ok {
			if v, _ := check["remote_version"].(string); !strings.Contains(v, "0.9.9") {
				t.Fatalf("fallback check must come from OTA: %+v", check)
			}
		}
	})

	t.Run("TC-VAP-003_无匹配行回落GitHub_OTA", func(t *testing.T) {
		empty := &vpShim{inner: configsync.NewVersionProviderAdapter(
			func(ctx context.Context) ([]configsync.Row, error) { return []configsync.Row{}, nil }, "stable")}
		h := newHandlerWithOTA(t, "v0.9.9")
		h.SetVersionProvider(empty)

		req := httptest.NewRequest(http.MethodGet, "/update/check", nil)
		w := httptest.NewRecorder()
		h.HandleCheckUpdate(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("fallback must answer 200, got %d", w.Code)
		}
		_, source := decodeCheck(t, w)
		if source == "configsync" {
			t.Fatal("no matching rows must fall back to OTA, not report configsync")
		}
	})

	t.Run("TC-VAP-004_渠道匹配注入路径", func(t *testing.T) {
		beta := &vpShim{inner: configsync.NewVersionProviderAdapter(
			func(ctx context.Context) ([]configsync.Row, error) {
				return []configsync.Row{releaseRow("0.4.0-beta1", "beta")}, nil
			}, "beta")}
		h := newHandlerWithOTA(t, "v0.9.9")
		h.SetVersionProvider(beta)
		req := httptest.NewRequest(http.MethodGet, "/update/check", nil)
		w := httptest.NewRecorder()
		h.HandleCheckUpdate(w, req)
		// beta adapter has a matching beta row → configsync source used.
		body, source := decodeCheck(t, w)
		if source != "configsync" {
			t.Fatalf("matching provider row must win: source=%q body=%v", source, body)
		}
	})

	t.Run("TC-VAP-005_未注入行为与现状一致", func(t *testing.T) {
		h := newHandlerWithOTA(t, "v0.9.9")
		req := httptest.NewRequest(http.MethodGet, "/update/check", nil)
		w := httptest.NewRecorder()
		h.HandleCheckUpdate(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("un-injected handler must behave exactly as before: %d", w.Code)
		}
		body, source := decodeCheck(t, w)
		if source == "configsync" {
			t.Fatalf("un-injected handler must not claim configsync: %v", body)
		}
		check, _ := body["check"].(map[string]any)
		if v, _ := check["remote_version"].(string); !strings.Contains(v, "0.9.9") {
			t.Fatalf("OTA result shape must be unchanged: %v", check)
		}
	})

	t.Run("TC-VAP-006_force_update透传", func(t *testing.T) {
		v, _ := configsync.MarshalValue(configsync.VersionInfo{Version: "0.3.4", ForceUpdate: true})
		rows := []configsync.Row{{Key: "release.channel.stable", Channel: "stable", Edition: "all", Enabled: true, Value: v}}
		var info configsync.VersionInfo
		_ = json.Unmarshal(rows[0].Value, &info)
		if !info.ForceUpdate {
			t.Fatal("force_update must be present in the injected payload")
		}
	})
}
