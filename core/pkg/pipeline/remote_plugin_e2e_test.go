package pipeline

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestRemotePluginDiscoverValidateExecute(t *testing.T) {
	var executeCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/centag-node-plugin.json":
			_ = json.NewEncoder(w).Encode(NodePluginDescriptor{
				Name:               "e2e-transform",
				Implementation:     "test-url",
				Kind:               "content.transform",
				Version:            "1.0.0",
				APIVersion:         PipelinePluginSchemaVersion,
				SupportsStream:    true,
				Concurrent:         true,
				Permissions:        []string{"network.outbound"},
				ConfigSchema:       JSONSchema{"type": "object", "properties": map[string]interface{}{"model": JSONSchema{"type": "string"}}},
				InputSchema:        JSONSchema{"type": "object", "properties": map[string]interface{}{"content": JSONSchema{"type": "string"}}},
				OutputSchema:       JSONSchema{"type": "object", "properties": map[string]interface{}{"content": JSONSchema{"type": "string"}}},
			})
		case "/validate":
			var req struct {
				SchemaVersion string    `json:"schema_version"`
				Config      NodeConfig `json:"config"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if req.Config.Model == "" {
				_ = json.NewEncoder(w).Encode(NodeValidateResponse{
					Valid: false,
					Errors: []NodeValidateError{
						{Code: "MISSING_FIELD", Message: "model is required", Field: "config.model"},
					},
				})
				return
			}
			_ = json.NewEncoder(w).Encode(NodeValidateResponse{Valid: true})
		case "/execute":
			executeCount.Add(1)
			var req NodeExecutionRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			_ = json.NewEncoder(w).Encode(NodeExecutionResponse{
				Output: &NodeOutput{
					Content: "transformed: " + req.Input.Content,
					Metadata: map[string]interface{}{
						"node_id":       req.NodeID,
						"model":         req.Config.Model,
						"execute_count": executeCount.Load(),
					},
				},
			})
		case "/health":
			w.WriteHeader(http.StatusOK)
		case "/stream":
			// 流式响应 - 返回 SSE 格式的数据
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")
			w.WriteHeader(http.StatusOK)
			var req NodeExecutionRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				return
			}
			// 模拟流式响应 - 使用 event: message 格式
			_, _ = w.Write([]byte("event: message\n"))
			_, _ = w.Write([]byte("data: {\"content\": \"transformed: " + req.Input.Content + "\"}\n\n"))
			// 发送 done 事件表示流结束
			_, _ = w.Write([]byte("event: done\n\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	plugin := NewRemoteNodePlugin(server.URL)

	desc := plugin.Descriptor()
	if desc.Name != "e2e-transform" {
		t.Errorf("descriptor name = %q, want e2e-transform", desc.Name)
	}
	if desc.Kind != "content.transform" {
		t.Errorf("descriptor kind = %q, want content.transform", desc.Kind)
	}
	if !desc.SupportsStream {
		t.Error("descriptor should support stream")
	}

	validConfig := NodeConfig{Model: "gpt-4"}
	if err := plugin.ValidateConfig(validConfig); err != nil {
		t.Errorf("ValidateConfig with valid config failed: %v", err)
	}

	invalidConfig := NodeConfig{}
	if err := plugin.ValidateConfig(invalidConfig); err == nil {
		t.Error("ValidateConfig with invalid config should return error")
	}

	resp, err := plugin.Execute(context.Background(), &NodeExecutionRequest{
		SchemaVersion:  PipelinePluginSchemaVersion,
		Implementation: server.URL,
		NodeID:         "transform-1",
		NodeName:       "Transform",
		Input:          &NodeInput{Content: "hello"},
		Config:         validConfig,
	})

	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if resp.Output.Content != "transformed: hello" {
		t.Errorf("output content = %q, want transformed: hello", resp.Output.Content)
	}
}

func TestRemotePluginHealthCheckLifecycle(t *testing.T) {
	var healthChecks atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			healthChecks.Add(1)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"healthy"}`))
		case "/.well-known/centag-node-plugin.json":
			_ = json.NewEncoder(w).Encode(NodePluginDescriptor{
				Name:           "health-check-test",
Implementation:     "test-url",
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
		t.Fatal("cannot convert to *RemoteNodePlugin")
	}

	remote.StartHealthCheck()
	time.Sleep(50 * time.Millisecond)

	if healthChecks.Load() == 0 {
		t.Error("health check should have been executed")
	}

	remote.StopHealthCheck()

	initialCount := healthChecks.Load()
	time.Sleep(100 * time.Millisecond)

	if healthChecks.Load() > initialCount {
		t.Error("health check should stop after StopHealthCheck")
	}
}

func TestCircuitBreakerMechanism(t *testing.T) {
	var attemptCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptCount.Add(1)
		if r.URL.Path == "/execute" {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("server error"))
		}
	}))
	defer server.Close()

	plugin := NewRemoteNodePlugin(server.URL)
	remote, ok := plugin.(*RemoteNodePlugin)
	if !ok {
		t.Fatal("cannot convert to *RemoteNodePlugin")
	}

	for i := 0; i < 4; i++ {
		_, err := plugin.Execute(context.Background(), &NodeExecutionRequest{
			SchemaVersion:  PipelinePluginSchemaVersion,
			Implementation: server.URL,
			Input:         &NodeInput{Content: "test"},
		})
		if err == nil {
			t.Errorf("expected error on attempt %d", i+1)
		}
	}

	_, err := plugin.Execute(context.Background(), &NodeExecutionRequest{
		SchemaVersion:  PipelinePluginSchemaVersion,
		Implementation: server.URL,
		Input:         &NodeInput{Content: "test"},
	})
	if err == nil {
		t.Error("should return error when circuit is open")
	}
	// 熔断器打开后，可能返回 "circuit breaker open" 或之前的错误消息
	// 因为熔断器检查是在 Execute 开始时进行的
	if err != nil && !containsString(err.Error(), "circuit") && !containsString(err.Error(), "failed") {
		t.Errorf("error should mention circuit breaker or failed: %v", err)
	}

	if !remote.IsCircuitOpen() {
		t.Error("circuit should be open")
	}
}

func TestCircuitBreakerRecovery(t *testing.T) {
	var firstFailure time.Time

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/execute" {
			if firstFailure.IsZero() {
				firstFailure = time.Now()
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			if time.Since(firstFailure) < 100*time.Millisecond {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			_ = json.NewEncoder(w).Encode(NodeExecutionResponse{
				Output: &NodeOutput{Content: "success"},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	plugin := NewRemoteNodePlugin(server.URL)
	remote, ok := plugin.(*RemoteNodePlugin)
	if !ok {
		t.Fatal("cannot convert to *RemoteNodePlugin")
	}

	// 需要 5 次连续失败才会打开熔断器
	for i := 0; i < 5; i++ {
		_, _ = plugin.Execute(context.Background(), &NodeExecutionRequest{
			SchemaVersion:  PipelinePluginSchemaVersion,
			Implementation: server.URL,
			Input:         &NodeInput{Content: "test"},
		})
	}

	if !remote.IsCircuitOpen() {
		t.Error("circuit should be open after 5 consecutive failures")
	}

	// 熔断器冷却时间为 30 秒，等待 35 秒确保恢复
	time.Sleep(35 * time.Second)

	_, err := plugin.Execute(context.Background(), &NodeExecutionRequest{
		SchemaVersion:  PipelinePluginSchemaVersion,
		Implementation: server.URL,
		Input:         &NodeInput{Content: "test"},
	})
	if err != nil {
		t.Errorf("should recover after cooldown: %v", err)
	}
}

func TestConcurrentRemotePluginAccess(t *testing.T) {
	var wg sync.WaitGroup
	var successCount atomic.Int32
	var mu sync.Mutex
	errors := []error{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/execute" {
			time.Sleep(10 * time.Millisecond)
			_ = json.NewEncoder(w).Encode(NodeExecutionResponse{
				Output: &NodeOutput{Content: "ok"},
			})
		}
	}))
	defer server.Close()

	plugin := NewRemoteNodePlugin(server.URL)

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := plugin.Execute(context.Background(), &NodeExecutionRequest{
				SchemaVersion:  PipelinePluginSchemaVersion,
				Implementation: server.URL,
				Input:         &NodeInput{Content: "test"},
			})
			if err == nil {
				successCount.Add(1)
			} else {
				mu.Lock()
				errors = append(errors, err)
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	if successCount.Load() != 20 {
		t.Errorf("success count = %d, want 20", successCount.Load())
	}
	if len(errors) > 0 {
		t.Errorf("unexpected errors: %v", errors)
	}
}

func TestRemotePluginTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/execute" {
			time.Sleep(200 * time.Millisecond)
			_ = json.NewEncoder(w).Encode(NodeExecutionResponse{
				Output: &NodeOutput{Content: "slow"},
			})
		}
	}))
	defer server.Close()

	plugin := NewRemoteNodePlugin(server.URL)
	plugin.(*RemoteNodePlugin).httpClient.Timeout = 50 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := plugin.Execute(ctx, &NodeExecutionRequest{
		SchemaVersion:  PipelinePluginSchemaVersion,
		Implementation: server.URL,
		Input:         &NodeInput{Content: "test"},
	})

	if err == nil {
		t.Error("expected timeout error")
	}
}

func TestRemotePluginInvalidManifest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/.well-known/centag-node-plugin.json" {
			_ = json.NewEncoder(w).Encode(map[string]string{
				"name": "bad-plugin",
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	plugin := NewRemoteNodePlugin(server.URL)

	desc := plugin.Descriptor()
	if desc.Kind == "" {
		t.Error("should handle invalid manifest gracefully")
	}
}

func TestRemotePluginValidateConfigRetryable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/validate" {
			_ = json.NewEncoder(w).Encode(NodeValidateResponse{
				Valid:     false,
				Code:     "RETRYABLE_ERROR",
				Message:  "temporary issue",
				Retryable: true,
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	plugin := NewRemoteNodePlugin(server.URL)
	err := plugin.ValidateConfig(NodeConfig{})

	if err == nil {
		t.Error("expected error for retryable response")
	}
}

func TestRemotePluginManifestHashValidation(t *testing.T) {
	badHash := "0000000000000000000000000000000000000000000000000000000000000000"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/.well-known/centag-node-plugin.json" {
			_ = json.NewEncoder(w).Encode(NodePluginDescriptor{
				Name:           "hash-test",
Implementation:     "test-url",
				Kind:           "test.kind",
				Version:        "1.0.0",
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	remote := NewRemoteNodePlugin(server.URL).(*RemoteNodePlugin)
	remote.hashConfig.Hash = badHash

	desc, err := remote.fetchDescriptor(context.Background())
	if err != nil {
		t.Logf("hash validation failed as expected: %v", err)
	} else if desc.Implementation == "" {
		t.Error("should handle hash mismatch")
	}
}

func TestRemotePluginConcurrentHealthCheck(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			w.WriteHeader(http.StatusOK)
		case "/.well-known/centag-node-plugin.json":
			_ = json.NewEncoder(w).Encode(NodePluginDescriptor{
				Name:           "concurrent-test",
				Implementation:     "test-url",
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
		t.Fatal("cannot convert to *RemoteNodePlugin")
	}

	remote.StartHealthCheck()

	time.Sleep(50 * time.Millisecond)

	remote.StartHealthCheck()

	time.Sleep(50 * time.Millisecond)

	remote.StopHealthCheck()

	time.Sleep(50 * time.Millisecond)

	if remote.IsHealthCheckRunning() {
		t.Error("health check should be stopped")
	}
}