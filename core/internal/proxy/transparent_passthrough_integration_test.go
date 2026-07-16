package proxy

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"centag/core/pkg/pipeline"
	"centag/core/pkg/plugin"
	openai "centag/plugins/protocol/openai"

	"github.com/gin-gonic/gin"
)

// mockTransparentHTTPClient records the upstream request for transparent_forward.
type mockTransparentHTTPClient struct {
	lastReq *http.Request
	body    string
	status  int
}

func (m *mockTransparentHTTPClient) Do(req *http.Request) (*http.Response, error) {
	m.lastReq = req
	return &http.Response{
		StatusCode: m.status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(m.body)),
	}, nil
}

type mockTransparentBroker struct {
	httpClient pipeline.HTTPClient
}

func (m *mockTransparentBroker) GetLLMClient(context.Context, []string) (pipeline.LLMClient, error) {
	return nil, nil
}
func (m *mockTransparentBroker) GetLLMStreamClient(context.Context, []string) (pipeline.LLMStreamClient, error) {
	return nil, nil
}
func (m *mockTransparentBroker) GetStorage(context.Context, []string) (pipeline.Storage, error) {
	return nil, nil
}
func (m *mockTransparentBroker) GetMemory(context.Context, []string) (pipeline.Memory, error) {
	return nil, nil
}
func (m *mockTransparentBroker) GetSecretsResolver(context.Context, []string) (pipeline.SecretsResolver, error) {
	return nil, nil
}
func (m *mockTransparentBroker) GetHTTPClient(context.Context, []string) (pipeline.HTTPClient, error) {
	return m.httpClient, nil
}
func (m *mockTransparentBroker) GetCacheStrategy(context.Context, string, []string) (pipeline.CacheStrategyCapability, error) {
	return nil, nil
}
func (m *mockTransparentBroker) GetVectorCache(context.Context, []string) (pipeline.VectorCacheCapability, error) {
	return nil, nil
}
func (m *mockTransparentBroker) GetEmbeddingService(context.Context, []string) (pipeline.EmbeddingCapability, error) {
	return nil, nil
}

// TestTransparentPassthrough_HandlerDispatcherNode verifies the #t path preserves
// scripts/tools/stream fields from handler body cache → ModeDispatcher metadata → transparent_forward upstream.
func TestTransparentPassthrough_HandlerDispatcherNode(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rawJSON := `{"model":"gpt-4","messages":[{"role":"user","content":"hi"}],"tools":[{"type":"function","function":{"name":"get_weather"}}],"tool_choice":"auto","stream":true}`

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(rawJSON))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("Authorization", "Bearer sk-test")
	c.Request.Header.Set("X-Target-URL", "https://api.example.com/v1/chat/completions")

	// Handler step: cache body before protocol parse.
	cacheRawRequestBody(c)

	proto := &openai.Protocol{}
	req, err := proto.ParseRequest(c)
	if err != nil {
		t.Fatalf("ParseRequest: %v", err)
	}
	rawBody, ok := req.RawBody.(map[string]interface{})
	if !ok {
		t.Fatalf("RawBody type = %T, want map", req.RawBody)
	}
	if _, ok := rawBody["tools"]; !ok {
		t.Fatal("parsed RawBody missing tools")
	}
	if stream, _ := rawBody["stream"].(bool); !stream {
		t.Fatal("parsed RawBody stream=false, want true")
	}

	dispatcher := NewModeDispatcher(&stubPipelineEngine{ids: map[string]bool{"transparent-proxy": true}}, nil, nil)
	input := dispatcher.buildPipelineInput(c, &plugin.ProxyRequest{
		Model:    req.Model,
		Messages: req.Messages,
		Stream:   req.Stream,
		RawBody:  req.RawBody,
	}, ModeTransparentProxy)

	gotBody, ok := input.Metadata["raw_request_body"].(string)
	if !ok || gotBody == "" {
		t.Fatalf("pipeline metadata missing raw_request_body, metadata=%v", input.Metadata)
	}
	if !strings.Contains(gotBody, `"tools"`) || !strings.Contains(gotBody, `"stream":true`) {
		t.Fatalf("raw_request_body lost scripts/tools/stream: %s", gotBody)
	}
	if gotBody != rawJSON {
		t.Fatalf("raw_request_body changed:\n got:  %s\n want: %s", gotBody, rawJSON)
	}

	// Pipeline transparent_forward node: forward bytes unchanged to upstream.
	httpClient := &mockTransparentHTTPClient{status: 200, body: `{"id":"upstream"}`}
	broker := &mockTransparentBroker{httpClient: httpClient}

	node, err := pipeline.NewTransparentForwardNode(pipeline.NodeConfig{})
	if err != nil {
		t.Fatalf("NewTransparentForwardNode: %v", err)
	}
	tf := node.(*pipeline.TransparentForwardNode)
	tf.SetCapabilityBroker(broker)

	output, err := tf.Execute(context.Background(), &pipeline.NodeInput{
		Metadata: input.Metadata,
	})
	if err != nil {
		t.Fatalf("transparent_forward Execute: %v", err)
	}
	if output.Content != `{"id":"upstream"}` {
		t.Fatalf("upstream response = %q", output.Content)
	}
	if httpClient.lastReq == nil {
		t.Fatal("upstream request not sent")
	}
	forwarded, err := io.ReadAll(httpClient.lastReq.Body)
	if err != nil {
		t.Fatalf("read forwarded body: %v", err)
	}
	if string(forwarded) != rawJSON {
		t.Fatalf("forwarded body = %s, want %s", string(forwarded), rawJSON)
	}

	// Sanity: re-marshal RawBody map would differ from original; passthrough must use cached bytes.
	var remarshaled map[string]interface{}
	if err := json.Unmarshal(forwarded, &remarshaled); err != nil {
		t.Fatalf("forwarded JSON invalid: %v", err)
	}
	if remarshaled["tool_choice"] != "auto" {
		t.Fatalf("tool_choice = %v", remarshaled["tool_choice"])
	}
}