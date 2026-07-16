package pipeline

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNodeRegistryCreatesBuiltinFromLegacyType(t *testing.T) {
	registry := NewNodeRegistry()
	if err := RegisterBuiltinNodes(registry); err != nil {
		t.Fatalf("RegisterBuiltinNodes failed: %v", err)
	}

	node, err := registry.CreateFromConfig(PipelineNodeConfig{
		ID:      "generate",
		Type:    NodeTypeGenerator,
		Name:    "Generate",
		Backend: "test-backend",
		Model:   "test-model",
	}, NodeConfig{Backend: "test-backend", Model: "test-model"})
	if err != nil {
		t.Fatalf("CreateFromConfig failed: %v", err)
	}
	if node.ID() != "generate" {
		t.Fatalf("expected node id to be preserved, got %q", node.ID())
	}
	if node.Type() != NodeTypeGenerator {
		t.Fatalf("expected generator type, got %q", node.Type())
	}
}

func TestAgentPatternPipelineAllowsImplementationNode(t *testing.T) {
	p := &AgentPatternPipeline{
		ID:   "plugin-pipeline",
		Name: "Plugin Pipeline",
		Nodes: []PipelineNodeConfig{
			{
				ID:             "remote",
				Kind:           "content.transform",
				Implementation: "https://plugin.example.com/node",
				Name:           "Remote Transform",
			},
		},
	}
	if err := p.Validate(); err != nil {
		t.Fatalf("expected plugin node to validate without backend/model: %v", err)
	}
}

func TestRemoteNodePluginExecute(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/centag-node-plugin.json":
			_ = json.NewEncoder(w).Encode(NodePluginDescriptor{
				Name:           "mock",
				Implementation: serverURL(r),
				Kind:           "content.transform",
				Version:        "1.0.0",
				Concurrent:     true,
			})
		case "/execute":
			_ = json.NewEncoder(w).Encode(NodeExecutionResponse{
				Output: &NodeOutput{
					Content: "remote output",
					Metadata: map[string]interface{}{
						"source": "mock",
					},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	plugin := NewRemoteNodePlugin(server.URL)
	resp, err := plugin.Execute(context.Background(), &NodeExecutionRequest{
		SchemaVersion:  PipelinePluginSchemaVersion,
		Implementation: server.URL,
		Input:          &NodeInput{Content: "input"},
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if resp.Output.Content != "remote output" {
		t.Fatalf("unexpected content: %q", resp.Output.Content)
	}
}

func TestAggregatorAcceptsMergeStrategy(t *testing.T) {
	node, err := NewAggregatorNode(NodeConfig{
		CustomConfig: map[string]interface{}{"strategy": "merge"},
	})
	if err != nil {
		t.Fatalf("NewAggregatorNode failed: %v", err)
	}
	if err := node.Validate(); err != nil {
		t.Fatalf("merge strategy should be valid: %v", err)
	}
}

func TestRemoteNodePluginValidateConfigValid(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/validate" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(NodeValidateResponse{Valid: true})
	}))
	defer server.Close()

	plugin := NewRemoteNodePlugin(server.URL)
	if err := plugin.ValidateConfig(NodeConfig{}); err != nil {
		t.Errorf("expected nil error for valid response, got %v", err)
	}
}

func TestRemoteNodePluginValidateConfigErrorsArray(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/validate" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(NodeValidateResponse{
			Valid: false,
			Errors: []NodeValidateError{
				{Code: "MISSING_FIELD", Message: "field 'model' is required", Field: "config.model"},
				{Code: "INVALID_ENUM", Message: "temperature must be between 0 and 2", Field: "config.temperature"},
			},
		})
	}))
	defer server.Close()

	plugin := NewRemoteNodePlugin(server.URL)
	err := plugin.ValidateConfig(NodeConfig{})
	if err == nil {
		t.Fatal("expected non-nil error for invalid response")
	}
	errMsg := err.Error()
	if !contains(errMsg, "MISSING_FIELD") || !contains(errMsg, "model") {
		t.Errorf("error message should contain field details: %s", errMsg)
	}
	if !contains(errMsg, "INVALID_ENUM") || !contains(errMsg, "temperature") {
		t.Errorf("error message should contain second error: %s", errMsg)
	}
}

func TestRemoteNodePluginValidateConfigCodeMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/validate" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(NodeValidateResponse{
			Valid:     false,
			Code:      "INVALID_CONFIG",
			Message:   "backend 'unknown' not found",
			Retryable: false,
		})
	}))
	defer server.Close()

	plugin := NewRemoteNodePlugin(server.URL)
	err := plugin.ValidateConfig(NodeConfig{})
	if err == nil {
		t.Fatal("expected non-nil error")
	}
	errMsg := err.Error()
	if !contains(errMsg, "INVALID_CONFIG") || !contains(errMsg, "backend") {
		t.Errorf("error should contain code and message: %s", errMsg)
	}
}

func TestRemoteNodePluginValidateConfigHTTP500Fallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/validate" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal error"))
	}))
	defer server.Close()

	plugin := NewRemoteNodePlugin(server.URL)
	err := plugin.ValidateConfig(NodeConfig{})
	if err == nil {
		t.Fatal("expected non-nil error for HTTP 500")
	}
	errMsg := err.Error()
	if !contains(errMsg, "500") {
		t.Errorf("error should contain status code: %s", errMsg)
	}
}

func TestRemoteNodePluginValidateConfigHTTPRedirect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/validate" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusMovedPermanently)
	}))
	defer server.Close()

	plugin := NewRemoteNodePlugin(server.URL)
	if err := plugin.ValidateConfig(NodeConfig{}); err != nil {
		t.Errorf("expected nil error on redirect, got %v", err)
	}
}

func TestRemoteNodePluginValidateConfigNotImplemented(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/.well-known/centag-node-plugin.json" {
			_ = json.NewEncoder(w).Encode(NodePluginDescriptor{Name: "mock", Implementation: serverURL(r)})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	plugin := NewRemoteNodePlugin(server.URL)
	if err := plugin.ValidateConfig(NodeConfig{}); err != nil {
		t.Errorf("expected nil error when /validate not implemented, got %v", err)
	}
}

func serverURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func TestBuiltinNodePluginDescriptorAPIVersion(t *testing.T) {
	registry := NewNodeRegistry()
	if err := RegisterBuiltinNodes(registry); err != nil {
		t.Fatalf("RegisterBuiltinNodes failed: %v", err)
	}

	for _, impl := range []string{"builtin.generator", "builtin.processor", "builtin.reviewer", "builtin.router", "builtin.aggregator"} {
		plugin, ok := registry.GetPlugin(impl)
		if !ok {
			t.Errorf("plugin %q not found", impl)
			continue
		}
		desc := plugin.Descriptor()
		if desc.APIVersion == "" {
			t.Errorf("plugin %q APIVersion is empty", impl)
		}
		if desc.APIVersion != PipelinePluginSchemaVersion {
			t.Errorf("plugin %q APIVersion = %q, want %q", impl, desc.APIVersion, PipelinePluginSchemaVersion)
		}
		if desc.MinCentagVersion != "" {
			t.Errorf("builtin plugin %q should not have MinCentagVersion set by default", impl)
		}
		if desc.Deprecated {
			t.Errorf("builtin plugin %q should not be deprecated by default", impl)
		}
		if len(desc.Tags) != 0 {
			t.Errorf("builtin plugin %q should not have tags by default, got %v", impl, desc.Tags)
		}
	}
}

func TestNewBuiltinNodePluginSetsAPIVersionDefault(t *testing.T) {
	factory := func(config NodeConfig) (PipelineNode, error) {
		return &BaseNode{id: "test", name: "Test"}, nil
	}
	plugin, err := NewBuiltinNodePlugin(NodeTypeGenerator, factory, NodePluginDescriptor{
		Name:    "test",
		Version: "1.0.0",
	})
	if err != nil {
		t.Fatalf("NewBuiltinNodePlugin failed: %v", err)
	}
	desc := plugin.Descriptor()
	if desc.APIVersion != PipelinePluginSchemaVersion {
		t.Errorf("APIVersion = %q, want %q", desc.APIVersion, PipelinePluginSchemaVersion)
	}
}

func TestNodePluginDescriptorTagsAndDeprecation(t *testing.T) {
	desc := NodePluginDescriptor{
		Name:               "test-plugin",
		Implementation:     "builtin.test",
		Kind:               "test.kind",
		Version:            "1.0.0",
		APIVersion:         PipelinePluginSchemaVersion,
		MinCentagVersion: "1.0.0",
		Deprecated:         true,
		Tags:               []string{"experimental", "llm"},
	}
	if !desc.Deprecated {
		t.Error("Deprecated should be true")
	}
	if len(desc.Tags) != 2 {
		t.Errorf("Tags length = %d, want 2", len(desc.Tags))
	}
	if desc.Tags[0] != "experimental" {
		t.Errorf("Tags[0] = %q, want experimental", desc.Tags[0])
	}
	if desc.MinCentagVersion != "1.0.0" {
		t.Errorf("MinCentagVersion = %q, want 1.0.0", desc.MinCentagVersion)
	}
}

func TestRemoteNodePluginDescriptorHasAPIVersion(t *testing.T) {
	plugin := NewRemoteNodePlugin("https://example.com/plugin")
	desc := plugin.Descriptor()
	if desc.APIVersion != PipelinePluginSchemaVersion {
		t.Errorf("APIVersion = %q, want %q", desc.APIVersion, PipelinePluginSchemaVersion)
	}
	if desc.MinCentagVersion == "" {
		t.Error("MinCentagVersion should be set for remote plugin")
	}
	if desc.Kind != "remote.node" {
		t.Errorf("Kind = %q, want remote.node", desc.Kind)
	}
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
