package registry

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRegistryHandlerMinimalLoop(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store := NewMemoryStore()
	handler := NewHandler(store)
	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)

	requestBody := RegisterPluginRequest{
		Name:        "hello-world",
		Version:     "1.0.0",
		Description: "Example plugin",
		Author:      "Proxyclaw",
		Category:    "example",
		Tags:        []string{"example"},
		Permissions: []string{"llm.call"},
		DownloadURL: "https://example.com/hello-world.zip",
		Checksum:    "sha256-demo",
		Size:        128,
	}
	body, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/registry/plugins", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("register status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var registerResp RegisterPluginResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &registerResp); err != nil {
		t.Fatalf("decode register response: %v", err)
	}
	if registerResp.ID != "hello-world@1.0.0" {
		t.Fatalf("registered id = %q", registerResp.ID)
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/registry/plugins?page=1&page_size=10", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var listResp ListPluginsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if listResp.Total != 1 || listResp.Plugins[0].ID != registerResp.ID {
		t.Fatalf("unexpected list response: %+v", listResp)
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/registry/plugins/"+registerResp.ID+"/versions/1.0.0", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("get version status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var versionResp PluginMetadata
	if err := json.Unmarshal(rec.Body.Bytes(), &versionResp); err != nil {
		t.Fatalf("decode version response: %v", err)
	}
	if versionResp.Name != requestBody.Name || versionResp.Version != requestBody.Version {
		t.Fatalf("unexpected version response: %+v", versionResp)
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/api/v1/registry/plugins/"+registerResp.ID, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("delete status = %d, body = %s", rec.Code, rec.Body.String())
	}
}
