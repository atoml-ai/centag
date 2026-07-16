package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"centag/core/pkg/pipeline"
)

type stubEngine struct {
	lastID    string
	lastInput *pipeline.PipelineInput
}

func (s *stubEngine) Execute(ctx context.Context, pipelineID string, input *pipeline.PipelineInput) (*pipeline.PipelineOutput, error) {
	s.lastID = pipelineID
	s.lastInput = input
	return &pipeline.PipelineOutput{Content: "ok"}, nil
}

func (s *stubEngine) GetPipelineConfig(pipelineID string) *pipeline.AgentPatternPipeline {
	return &pipeline.AgentPatternPipeline{
		ID:           pipelineID,
		GlobalConfig: pipeline.GlobalPipelineConfig{Timeout: 30},
	}
}

func TestTriggerPipeline_WithSecret(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := &stubEngine{}
	h := NewHandler(engine, "test-secret")

	body, _ := json.Marshal(triggerRequest{Content: "deploy finished"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/pipeline/ci-flow", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Secret", "test-secret")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Params = gin.Params{{Key: "id", Value: "ci-flow"}}

	h.TriggerPipeline(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if engine.lastID != "ci-flow" {
		t.Fatalf("pipeline id = %q", engine.lastID)
	}
	if engine.lastInput.Metadata["webhook_trigger"] != true {
		t.Fatal("expected webhook_trigger metadata")
	}
}

func TestTriggerPipeline_EmptySecretWithoutAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(&stubEngine{}, "")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/pipeline/x", bytes.NewReader([]byte(`{"content":"a"}`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Params = gin.Params{{Key: "id", Value: "x"}}

	h.TriggerPipeline(c)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 when secret unset and caller unauthenticated", w.Code)
	}
}

func TestTriggerPipeline_Unauthorized(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(&stubEngine{}, "secret")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/pipeline/x", bytes.NewReader([]byte(`{"content":"a"}`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Params = gin.Params{{Key: "id", Value: "x"}}

	h.TriggerPipeline(c)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}