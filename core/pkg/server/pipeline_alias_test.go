package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"centag/core/pkg/pipeline"

	"github.com/gin-gonic/gin"
)

// TestGetPipelineAcceptsLegacyAliasID 是 P1-T5 的回归：管理端读详情/验证
// 必须与执行层同源归一旧流水线 ID（direct-backend → transparent），
// 不再出现「执行可用、详情 403」的裂缝。
func TestGetPipelineAcceptsLegacyAliasID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	reg := pipeline.NewPipelineRegistry()
	if err := reg.Register(&pipeline.AgentPatternPipeline{ID: "transparent", Name: "transparent"}); err != nil {
		t.Fatalf("register transparent: %v", err)
	}

	h := NewPipelineHandler(nil, nil, reg, nil, nil)
	if h == nil {
		t.Fatal("NewPipelineHandler() = nil")
	}

	router := gin.New()
	router.GET("/api/v1/pipelines/:id", func(c *gin.Context) {
		c.Set("access_scope", "global")
		h.GetPipeline(c)
	})

	for _, id := range []string{"transparent", "direct-backend", "transparent-proxy"} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/pipelines/"+id, nil)
		router.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("GET pipeline %q: status = %d, want 200; body=%s", id, w.Code, w.Body.String())
		}
		var resp struct {
			Success bool `json:"success"`
			Data    struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode response for %q: %v", id, err)
		}
		if !resp.Success || resp.Data.ID != "transparent" {
			t.Fatalf("GET pipeline %q: got id=%q success=%v, want transparent", id, resp.Data.ID, resp.Success)
		}
	}
}
