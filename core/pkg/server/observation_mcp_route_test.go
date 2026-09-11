package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	mcppkg "centag/core/pkg/server/mcp"
)

// TestObservationMcpRouteServesAtStandardPath 复刻 setupRoutes 的观测面端点挂载方式，
// 断言标准路径 /v1/mcp（无尾斜杠）直接命中 handler，不依赖 Gin 尾斜杠重定向。
func TestObservationMcpRouteServesAtStandardPath(t *testing.T) {
	gin.SetMode(gin.TestMode)

	s := mcppkg.NewObservationServer(true, mcppkg.Deps{Version: "test", Authorize: func(*http.Request) bool { return true }}, nil)
	if s == nil {
		t.Fatal("observation server should be enabled")
	}

	router := gin.New()
	router.Handle(http.MethodGet, "/v1/mcp", gin.WrapF(s.StreamableHandler().ServeHTTP))
	router.Handle(http.MethodPost, "/v1/mcp", gin.WrapF(s.StreamableHandler().ServeHTTP))
	router.Handle(http.MethodDelete, "/v1/mcp", gin.WrapF(s.StreamableHandler().ServeHTTP))

	for _, tc := range []struct {
		method, path string
		wantNot      int // 不得返回的状态码
	}{
		{http.MethodGet, "/v1/mcp", http.StatusNotFound},
		{http.MethodPost, "/v1/mcp", http.StatusNotFound},
		{http.MethodDelete, "/v1/mcp", http.StatusNotFound},
		{http.MethodGet, "/v1/mcp/", http.StatusNotFound},
	} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(tc.method, tc.path, nil)
		router.ServeHTTP(w, req)
		if w.Code == tc.wantNot {
			t.Fatalf("%s %s: returned %d (body=%s)", tc.method, tc.path, w.Code, w.Body.String())
		}
	}
}
