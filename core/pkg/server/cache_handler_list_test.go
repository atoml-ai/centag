package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"centag/core/internal/cache"
)

func newTestCacheHandler(t *testing.T) (*CacheHandler, *cache.Manager) {
	t.Helper()
	mgr, err := cache.NewManager(&cache.CacheConfig{Enabled: true, DefaultTTL: 3600})
	if err != nil {
		t.Fatal(err)
	}
	pc := cache.NewProxyCache(mgr, true)
	return NewCacheHandler(mgr, pc, nil), mgr
}

func seedExactEntry(t *testing.T, mgr *cache.Manager, key, session, model, request, response string) {
	t.Helper()
	exact := mgr.GetExactCache()
	if exact == nil {
		t.Fatal("exact cache nil")
	}
	err := exact.Set(context.Background(), key, &cache.CacheEntry{
		Key:       key,
		Request:   request,
		Response:  response,
		Timestamp: time.Now().UTC(),
		ExpiresAt: time.Now().Add(time.Hour),
		Metadata: map[string]interface{}{
			"session_id": session,
			"model":      model,
			"cache_type": "exact",
		},
	}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
}

func TestCacheHandler_ListGetDelete_MultiFilter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, mgr := newTestCacheHandler(t)
	seedExactEntry(t, mgr, "k-a", "sess-A", "qwen/qwen3", "hello world", "hi")
	seedExactEntry(t, mgr, "k-b", "sess-B", "gpt-4o", "other", "bye")

	r := gin.New()
	r.GET("/entries", h.ListCacheEntries)
	r.GET("/entry", h.GetCacheEntry)
	r.DELETE("/entry", h.DeleteCacheEntry)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/entries?type=exact&session_id=sess-A&model=qwen&q=hello&page=1&size=10", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", w.Code, w.Body.String())
	}
	var listResp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &listResp); err != nil {
		t.Fatal(err)
	}
	if listResp["success"] != true {
		t.Fatalf("list=%v", listResp)
	}
	data := listResp["data"].(map[string]interface{})
	if data["total_count"].(float64) != 1 {
		t.Fatalf("filtered total_count=%v data=%v", data["total_count"], data)
	}
	entries := data["entries"].([]interface{})
	if len(entries) != 1 {
		t.Fatalf("entries=%v", entries)
	}
	first := entries[0].(map[string]interface{})
	if first["session_id"] != "sess-A" {
		t.Fatalf("session_id=%v", first["session_id"])
	}

	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, httptest.NewRequest(http.MethodGet, "/entry?key=k-a&type=exact", nil))
	if w2.Code != http.StatusOK {
		t.Fatalf("get status=%d body=%s", w2.Code, w2.Body.String())
	}
	var getResp map[string]interface{}
	_ = json.Unmarshal(w2.Body.Bytes(), &getResp)
	got := getResp["data"].(map[string]interface{})
	if got["response"] != "hi" || got["cache_type"] != "exact" {
		t.Fatalf("detail=%v", got)
	}

	w3 := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/entry?key=k-a&type=exact", nil)
	r.ServeHTTP(w3, req)
	if w3.Code != http.StatusOK {
		t.Fatalf("delete status=%d body=%s", w3.Code, w3.Body.String())
	}

	w4 := httptest.NewRecorder()
	r.ServeHTTP(w4, httptest.NewRequest(http.MethodGet, "/entry?key=k-a", nil))
	if w4.Code != http.StatusNotFound {
		t.Fatalf("after delete want 404, got %d body=%s", w4.Code, w4.Body.String())
	}
}

func TestCacheHandler_GetCacheEntry_KeyRequired(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, _ := newTestCacheHandler(t)
	r := gin.New()
	r.GET("/entry", h.GetCacheEntry)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/entry", nil))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", w.Code)
	}
}

func TestCacheHandler_DeleteCacheEntry_KeyRequired(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, _ := newTestCacheHandler(t)
	r := gin.New()
	r.DELETE("/entry", h.DeleteCacheEntry)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/entry", nil))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", w.Code)
	}
}
