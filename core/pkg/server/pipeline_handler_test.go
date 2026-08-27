package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"centag/core/internal/auth"
	"centag/core/pkg/pipeline"

	"github.com/gin-gonic/gin"
)

type mockHTTPClient struct {
	lastReq *http.Request
	body    string
	status  int
}

func (m *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	m.lastReq = req
	return &http.Response{
		StatusCode: m.status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(m.body)),
	}, nil
}

type testCapabilityBroker struct {
	httpClient pipeline.HTTPClient
}

type testLogger struct{}

func (l *testLogger) Debug(msg string, fields ...interface{}) {}
func (l *testLogger) Info(msg string, fields ...interface{})  {}
func (l *testLogger) Warn(msg string, fields ...interface{})  {}
func (l *testLogger) Error(msg string, fields ...interface{}) {}

func (b *testCapabilityBroker) GetLLMClient(ctx context.Context, permissions []string) (pipeline.LLMClient, error) {
	return nil, nil
}
func (b *testCapabilityBroker) GetLLMStreamClient(ctx context.Context, permissions []string) (pipeline.LLMStreamClient, error) {
	return nil, nil
}
func (b *testCapabilityBroker) GetStorage(ctx context.Context, permissions []string) (pipeline.Storage, error) {
	return nil, nil
}
func (b *testCapabilityBroker) GetMemory(ctx context.Context, permissions []string) (pipeline.Memory, error) {
	return nil, nil
}
func (b *testCapabilityBroker) GetSecretsResolver(ctx context.Context, permissions []string) (pipeline.SecretsResolver, error) {
	return nil, nil
}
func (b *testCapabilityBroker) GetHTTPClient(ctx context.Context, permissions []string) (pipeline.HTTPClient, error) {
	if b.httpClient != nil {
		return b.httpClient, nil
	}
	return nil, nil
}
func (b *testCapabilityBroker) GetCacheStrategy(ctx context.Context, strategy string, permissions []string) (pipeline.CacheStrategyCapability, error) {
	return nil, nil
}
func (b *testCapabilityBroker) GetVectorCache(ctx context.Context, permissions []string) (pipeline.VectorCacheCapability, error) {
	return nil, nil
}
func (b *testCapabilityBroker) GetEmbeddingService(ctx context.Context, permissions []string) (pipeline.EmbeddingCapability, error) {
	return nil, nil
}

func buildTestSetup(t *testing.T, pipelineID string, mockStatus int, mockBody string, switchOn bool) (*gin.Engine, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	broker := &testCapabilityBroker{
		httpClient: &mockHTTPClient{
			status: mockStatus,
			body:   mockBody,
		},
	}

	nodeReg := pipeline.NewNodeRegistry()
	if err := pipeline.RegisterBuiltinNodes(nodeReg); err != nil {
		t.Fatalf("register builtin nodes: %v", err)
	}
	pipelineReg := pipeline.NewPipelineRegistry()
	p := &pipeline.AgentPatternPipeline{
		ID:      pipelineID,
		Name:    pipelineID,
		Version: "1.0",
		Nodes: []pipeline.PipelineNodeConfig{
			{
				ID:   "forward",
				Type: pipeline.NodeTypeTransparentForward,
				Config: pipeline.NodeConfig{
					Backend: "test-backend",
					Model:   "test-model",
				},
			},
		},
	}
	if err := pipelineReg.Register(p); err != nil {
		t.Fatalf("register: %v", err)
	}

	engine := pipeline.NewPipelineEngine(nodeReg, pipelineReg, broker, &testLogger{}, nil)
	handler := NewPipelineHandler(engine, nodeReg, pipelineReg, nil, nil)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(auth.CtxKeyRole, auth.RoleAdmin)
		c.Next()
	})
	router.POST("/api/v1/pipelines/:id/execute", handler.ExecutePipeline)

	if switchOn {
		os.Setenv("LLM_PROXY_TRANSPARENT_EXECUTE_PRESERVE_STATUS", "true")
	} else {
		os.Unsetenv("LLM_PROXY_TRANSPARENT_EXECUTE_PRESERVE_STATUS")
	}

	return router, httptest.NewRecorder()
}

func TestExecutePipeline_StatusPreserved_SwitchOn(t *testing.T) {
	router, w := buildTestSetup(t, "test-switch-on", http.StatusBadRequest,
		`{"error":"invalid request"}`, true)
	defer os.Unsetenv("LLM_PROXY_TRANSPARENT_EXECUTE_PRESERVE_STATUS")

	body, _ := json.Marshal(map[string]interface{}{
		"content": "hello",
		"metadata": map[string]interface{}{
			"backend_id":           "test-backend",
			"target_url":           "https://api.example.com",
			"request_path":         "/v1/chat/completions",
			"raw_request_body":     `{"model":"test-model","messages":[{"role":"user","content":"hello"}]}`,
			"forward_authorization": "Bearer sk-test",
		},
	})

	req, _ := http.NewRequest("POST", "/api/v1/pipelines/test-switch-on/execute", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d (body: %s)", http.StatusBadRequest, w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["success"] != false {
		t.Fatalf("expected success=false, got %v", resp["success"])
	}
}

func TestExecutePipeline_StatusPreserved_SwitchOff(t *testing.T) {
	router, w := buildTestSetup(t, "test-switch-off", http.StatusBadRequest,
		`{"error":"invalid request"}`, false)

	body, _ := json.Marshal(map[string]interface{}{
		"content": "hello",
		"metadata": map[string]interface{}{
			"backend_id":           "test-backend",
			"target_url":           "https://api.example.com",
			"request_path":         "/v1/chat/completions",
			"raw_request_body":     `{"model":"test-model","messages":[{"role":"user","content":"hello"}]}`,
			"forward_authorization": "Bearer sk-test",
		},
	})

	req, _ := http.NewRequest("POST", "/api/v1/pipelines/test-switch-off/execute", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d (body: %s)", http.StatusOK, w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["success"] != true {
		t.Fatalf("expected success=true, got %v", resp["success"])
	}
}

func TestExecutePipeline_200Normal(t *testing.T) {
	router, w := buildTestSetup(t, "test-200-normal", http.StatusOK,
		`{"choices":[{"message":{"content":"hi"}}]}`, true)
	defer os.Unsetenv("LLM_PROXY_TRANSPARENT_EXECUTE_PRESERVE_STATUS")

	body, _ := json.Marshal(map[string]interface{}{
		"content": "hello",
		"metadata": map[string]interface{}{
			"backend_id":           "test-backend",
			"target_url":           "https://api.example.com",
			"request_path":         "/v1/chat/completions",
			"raw_request_body":     `{"model":"test-model","messages":[{"role":"user","content":"hello"}]}`,
			"forward_authorization": "Bearer sk-test",
		},
	})

	req, _ := http.NewRequest("POST", "/api/v1/pipelines/test-200-normal/execute", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d (body: %s)", http.StatusOK, w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["success"] != true {
		t.Fatalf("expected success=true, got %v", resp["success"])
	}
}
