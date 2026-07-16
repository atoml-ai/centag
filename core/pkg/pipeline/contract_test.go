package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDescriptorContractValidation(t *testing.T) {
	tests := []struct {
		name        string
		descriptor  NodePluginDescriptor
		wantErr     bool
		errContains string
	}{
		{
			name: "valid descriptor passes",
			descriptor: NodePluginDescriptor{
				Name:           "test-plugin",
				Implementation: "builtin.test",
				Kind:           "test.kind",
				Version:        "1.0.0",
				APIVersion:     PipelinePluginSchemaVersion,
			},
			wantErr: false,
		},
		{
			name: "missing implementation fails",
			descriptor: NodePluginDescriptor{
				Name:   "test-plugin",
				Kind:   "test.kind",
				Version: "1.0.0",
			},
			wantErr:     true,
			errContains: "implementation",
		},
		{
			name: "missing kind fails",
			descriptor: NodePluginDescriptor{
				Name:           "test-plugin",
				Implementation: "builtin.test",
				Version:        "1.0.0",
			},
			wantErr:     true,
			errContains: "kind",
		},
		{
			name: "missing version fails",
			descriptor: NodePluginDescriptor{
				Name:           "test-plugin",
				Implementation: "builtin.test",
				Kind:           "test.kind",
			},
			wantErr:     true,
			errContains: "version",
		},
		{
			name: "valid remote descriptor",
			descriptor: NodePluginDescriptor{
				Name:           "remote-transform",
				Implementation: "https://plugin.example.com/node",
				Kind:           "content.transform",
				Version:        "1.0.0",
				Remote: &RemoteNodePluginSpec{
					BaseURL:     "https://plugin.example.com/node",
					ManifestURL: "https://plugin.example.com/node/.well-known/centag-node-plugin.json",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateManifest(&tt.descriptor)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateManifest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil && tt.errContains != "" {
				if !containsString(err.Error(), tt.errContains) {
					t.Errorf("error should contain %q, got %v", tt.errContains, err)
				}
			}
		})
	}
}

func TestDescriptorVersionFormat(t *testing.T) {
	versions := []string{
		"1.0.0",
		"1.0.0-alpha",
		"1.0.0-beta.1",
		"v1.0.0",
		"0.0.1",
		"2.0.0-RC1",
	}

	for _, version := range versions {
		desc := NodePluginDescriptor{
			Name:           "test",
			Implementation: "builtin.test",
			Kind:          "test.kind",
			Version:       version,
			APIVersion:    PipelinePluginSchemaVersion,
		}
		if err := validateManifest(&desc); err != nil {
			t.Errorf("version %q should be valid: %v", version, err)
		}
	}
}

func TestValidateContractErrorCodes(t *testing.T) {
	tests := []struct {
		name       string
		resp       NodeValidateResponse
		wantErr    bool
		errContains string
	}{
		{
			name: "valid response returns no error",
			resp: NodeValidateResponse{
				Valid: true,
			},
			wantErr: false,
		},
		{
			name: "invalid with single error",
			resp: NodeValidateResponse{
				Valid: false,
				Errors: []NodeValidateError{
					{Code: "MISSING_FIELD", Message: "field 'model' is required", Field: "config.model"},
				},
			},
			wantErr:    true,
			errContains: "MISSING_FIELD",
		},
		{
			name: "invalid with multiple errors",
			resp: NodeValidateResponse{
				Valid: false,
				Errors: []NodeValidateError{
					{Code: "MISSING_FIELD", Message: "field 'model' is required", Field: "config.model"},
					{Code: "INVALID_ENUM", Message: "temperature must be between 0 and 2", Field: "config.temperature"},
				},
			},
			wantErr:    true,
			errContains: "MISSING_FIELD",
		},
		{
			name: "invalid with code and message",
			resp: NodeValidateResponse{
				Valid:    false,
				Code:    "INVALID_CONFIG",
				Message: "backend 'unknown' not found",
			},
			wantErr:    true,
			errContains: "INVALID_CONFIG",
		},
		{
			name: "retryable error",
			resp: NodeValidateResponse{
				Valid:     false,
				Code:     "TEMPORARY_ERROR",
				Message:  "service temporarily unavailable",
				Retryable: true,
			},
			wantErr:    true,
			errContains: "TEMPORARY_ERROR",
		},
		{
			name: "details map included",
			resp: NodeValidateResponse{
				Valid: false,
				Errors: []NodeValidateError{
					{
						Code:    "VALIDATION_ERROR",
						Message: "validation failed",
						Details: map[string]interface{}{
							"min": 0,
							"max": 100,
						},
					},
				},
			},
			wantErr:    true,
			errContains: "VALIDATION_ERROR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			if !tt.resp.Valid {
				if len(tt.resp.Errors) > 0 {
					errMsgs := make([]string, 0, len(tt.resp.Errors))
					for _, e := range tt.resp.Errors {
						if e.Code != "" && e.Message != "" {
							errMsgs = append(errMsgs, e.Code+": "+e.Message)
						}
					}
					if len(errMsgs) > 0 {
						err = fmt.Errorf("validation failed: %s", joinStrings(errMsgs, "; "))
					}
				}
				if tt.resp.Message != "" && err == nil {
					err = fmt.Errorf("%s: %s", tt.resp.Code, tt.resp.Message)
				}
			}

			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil && tt.errContains != "" {
				if !containsString(err.Error(), tt.errContains) {
					t.Errorf("error should contain %q, got %v", tt.errContains, err)
				}
			}
		})
	}
}

func TestExecuteContractInputOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/centag-node-plugin.json":
			_ = json.NewEncoder(w).Encode(NodePluginDescriptor{
				Name:           "test-plugin",
				Implementation: "test-url",
				Kind:           "content.transform",
				Version:        "1.0.0",
				APIVersion:     PipelinePluginSchemaVersion,
			})
		case "/execute":
			var req NodeExecutionRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if req.SchemaVersion == "" {
				http.Error(w, "missing schema_version", http.StatusBadRequest)
				return
			}
			if req.Implementation == "" {
				http.Error(w, "missing implementation", http.StatusBadRequest)
				return
			}
			_ = json.NewEncoder(w).Encode(NodeExecutionResponse{
				Output: &NodeOutput{
					Content: "transformed: " + req.Input.Content,
					Metadata: map[string]interface{}{
						"input_len": len(req.Input.Content),
						"node_id":  req.NodeID,
					},
				},
				Events: []NodeExecutionEvent{
					{Type: "start", Message: "execution started"},
					{Type: "done", Message: "execution completed"},
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
		NodeID:         "test-node",
		Input:          &NodeInput{
			Content: "hello world",
			Metadata: map[string]interface{}{
				"timestamp": time.Now().Unix(),
			},
		},
		Context: map[string]interface{}{
			"pipeline_id": "test-pipeline",
			"user_id":    "user-123",
		},
	})

	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if resp.Output == nil {
		t.Fatal("Output should not be nil")
	}
	if resp.Output.Content != "transformed: hello world" {
		t.Errorf("unexpected content: %q", resp.Output.Content)
	}
	if len(resp.Events) != 2 {
		t.Errorf("expected 2 events, got %d", len(resp.Events))
	}
}

func TestExecuteContractErrorScenarios(t *testing.T) {
	tests := []struct {
		name          string
		handler       http.HandlerFunc
		wantErr       bool
		errContains   string
	}{
		{
			name: "network timeout",
			handler: func(w http.ResponseWriter, r *http.Request) {
				// 模拟一个永远不会响应的服务器
				// 使用 Hijack 获取底层连接，然后保持不响应
				hijacker, ok := w.(http.Hijacker)
				if !ok {
					return
				}
				conn, _, err := hijacker.Hijack()
				if err != nil {
					return
				}
				// 保持连接打开但不发送任何数据，直到客户端超时
				<-r.Context().Done()
				conn.Close()
			},
			wantErr: true,
		},
		{
			name: "500 server error",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"code":"INTERNAL_ERROR","message":"plugin crashed"}`))
			},
			wantErr:     true,
			errContains: "500",
		},
		{
			name: "403 forbidden",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(`{"code":"FORBIDDEN","message":"access denied"}`))
			},
			wantErr:     true,
			errContains: "403",
		},
		{
			name: "invalid JSON response",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte("not valid json"))
			},
			wantErr:     true,
			errContains: "decode",
		},
		{
			name: "empty output returned",
			handler: func(w http.ResponseWriter, r *http.Request) {
				_ = json.NewEncoder(w).Encode(NodeExecutionResponse{
					Output: nil,
				})
			},
			wantErr:     true,
			errContains: "empty output",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(tt.handler)
			defer server.Close()

			plugin := NewRemoteNodePlugin(server.URL)

			// 为 network timeout 测试添加超时 context
			ctx := context.Background()
			if tt.name == "network timeout" {
				timeoutCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()
				ctx = timeoutCtx
			}

			_, err := plugin.Execute(ctx, &NodeExecutionRequest{
				SchemaVersion:  PipelinePluginSchemaVersion,
				Implementation: server.URL,
				Input:         &NodeInput{Content: "test"},
			})

			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil && tt.errContains != "" {
				if !containsString(err.Error(), tt.errContains) {
					t.Errorf("error should contain %q, got %v", tt.errContains, err)
				}
			}
		})
	}
}

func TestNodeTypeContractKindMapping(t *testing.T) {
	tests := []struct {
		nodeType     NodeType
		expectedKind string
	}{
		{NodeTypeGenerator, "llm.generate"},
		{NodeTypeProcessor, "content.transform"},
		{NodeTypeReviewer, "quality.review"},
		{NodeTypeRouter, "route.decide"},
		{NodeTypeAggregator, "aggregate.merge"},
		{NodeTypeMemory, "memory.query"},
		{NodeTypeAudit, "audit.safety"},
		{NodeTypeOptimize, "optimize.enhance"},
	}

	for _, tt := range tests {
		t.Run(string(tt.nodeType), func(t *testing.T) {
			kind := KindForBuiltinType(tt.nodeType)
			if kind != tt.expectedKind {
				t.Errorf("KindForBuiltinType(%s) = %q, want %q", tt.nodeType, kind, tt.expectedKind)
			}
		})
	}
}

func TestBuiltinImplementationPrefix(t *testing.T) {
	impl := BuiltinImplementationForType(NodeTypeGenerator)
	if impl != "builtin.generator" {
		t.Errorf("BuiltinImplementationForType(generator) = %q, want builtin.generator", impl)
	}

	impl = BuiltinImplementationForType(NodeTypeProcessor)
	if impl != "builtin.processor" {
		t.Errorf("BuiltinImplementationForType(processor) = %q, want builtin.processor", impl)
	}
}

func TestIsRemoteImplementation(t *testing.T) {
	tests := []struct {
		impl    string
		wantRes bool
	}{
		{"https://plugin.example.com/node", true},
		{"http://localhost:8080/plugin", true},
		{"HTTP://example.com", true},
		{"builtin.generator", false},
		{"custom.plugin", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.impl, func(t *testing.T) {
			res := IsRemoteImplementation(tt.impl)
			if res != tt.wantRes {
				t.Errorf("IsRemoteImplementation(%q) = %v, want %v", tt.impl, res, tt.wantRes)
			}
		})
	}
}

func TestCircuitBreakerContract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	plugin := NewRemoteNodePlugin(server.URL)
	remote, ok := plugin.(*RemoteNodePlugin)
	if !ok {
		t.Fatalf("cannot convert to *RemoteNodePlugin")
	}

	for i := 0; i < 5; i++ {
		_, err := plugin.Execute(context.Background(), &NodeExecutionRequest{
			SchemaVersion:  PipelinePluginSchemaVersion,
			Implementation: server.URL,
			Input:         &NodeInput{Content: "test"},
		})
		if err == nil {
			t.Errorf("expected error on attempt %d", i+1)
		}
	}

	if !remote.IsCircuitOpen() {
		t.Error("circuit breaker should be open after 5 failures")
	}

	count := remote.GetFailureCount()
	if count < 5 {
		t.Errorf("failure count = %d, want >= 5", count)
	}
}

func TestHealthCheckLifecycle(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			w.WriteHeader(http.StatusOK)
		case "/.well-known/centag-node-plugin.json":
			_ = json.NewEncoder(w).Encode(NodePluginDescriptor{
				Name:           "test-plugin",
				Implementation: "test-url",
				Kind:           "test.kind",
				Version:        "1.0.0",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	plugin := NewRemoteNodePlugin(server.URL)
	remote, ok := plugin.(*RemoteNodePlugin)
	if !ok {
		t.Fatalf("cannot convert to *RemoteNodePlugin")
	}

	remote.StartHealthCheck()
	if !remote.IsHealthCheckRunning() {
		t.Error("health check should be running")
	}

	time.Sleep(100 * time.Millisecond)

	remote.StopHealthCheck()
	if remote.IsHealthCheckRunning() {
		t.Error("health check should be stopped")
	}
}

func TestNormalizeImplementation(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"  builtin.generator  ", "builtin.generator"},
		{"https://plugin.com", "https://plugin.com"},
		{"builtin.processor", "builtin.processor"},
		{"", ""},
		{"   ", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := NormalizeImplementation(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeImplementation(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func joinStrings(items []string, sep string) string {
	result := ""
	for i, item := range items {
		if i > 0 {
			result += sep
		}
		result += item
	}
	return result
}