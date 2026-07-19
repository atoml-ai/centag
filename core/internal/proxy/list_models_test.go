package proxy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"centag/core/pkg/pipeline"

	"github.com/gin-gonic/gin"
)

func TestListModelsReturnsPipelineIDs(t *testing.T) {
	gin.SetMode(gin.TestMode)

	reg := pipeline.NewPipelineRegistry()
	if err := reg.Register(&pipeline.AgentPatternPipeline{
		ID:   "direct-backend",
		Name: "直接后端",
	}); err != nil {
		t.Fatalf("register pipeline: %v", err)
	}
	if err := reg.Register(&pipeline.AgentPatternPipeline{
		ID:   "smart-scheduling",
		Name: "智能调度",
	}); err != nil {
		t.Fatalf("register pipeline: %v", err)
	}

	h := NewHandler(nil, nil, nil)
	h.SetPipelineRegistry(reg)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)

	h.ListModels(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, body=%s", w.Code, w.Body.String())
	}

	var resp struct {
		Object string `json:"object"`
		Data   []struct {
			ID      string `json:"id"`
			Object  string `json:"object"`
			OwnedBy string `json:"owned_by"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Object != "list" {
		t.Fatalf("object=%q, want list", resp.Object)
	}
	if len(resp.Data) != 2 {
		t.Fatalf("len(data)=%d, want 2: %+v", len(resp.Data), resp.Data)
	}

	got := map[string]bool{}
	for _, m := range resp.Data {
		got[m.ID] = true
		if m.Object != "model" {
			t.Errorf("id=%s object=%q, want model", m.ID, m.Object)
		}
		if m.OwnedBy != "centag" {
			t.Errorf("id=%s owned_by=%q, want centag", m.ID, m.OwnedBy)
		}
	}
	if !got["pipeline.direct-backend"] || !got["pipeline.smart-scheduling"] {
		t.Fatalf("unexpected models: %v", got)
	}
}

func TestListModelsEmptyWhenNoRegistry(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := NewHandler(nil, nil, nil)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)

	h.ListModels(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, body=%s", w.Code, w.Body.String())
	}

	var resp struct {
		Data []json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Data) != 0 {
		t.Fatalf("len(data)=%d, want 0", len(resp.Data))
	}
}
