package pipeline

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestRemotePluginCircuitBreaker 测试熔断机制
func TestRemotePluginCircuitBreaker(t *testing.T) {
	plugin := &RemoteNodePlugin{
		baseURL:    "http://example.com",
		httpClient: &http.Client{},
	}

	// 模拟失败
	for i := 0; i < 5; i++ {
		plugin.recordFailure()
	}

	if !plugin.IsCircuitOpen() {
		t.Error("Expected circuit to be open after 5 failures")
	}

	// 冷却时间未过，应该拒绝请求
	plugin.setLastFailureForTest(time.Now())
	if !plugin.IsCircuitOpen() {
		t.Error("Expected circuit still open")
	}

	// 模拟冷却时间已过
	plugin.setLastFailureForTest(time.Now().Add(-31 * time.Second))
	// 尝试执行，应该重置熔断
	plugin.resetFailure()
	if plugin.IsCircuitOpen() {
		t.Error("Expected circuit to be closed after cooldown")
	}
}

// TestSSEParsing 测试SSE流式解析
func TestSSEParsing(t *testing.T) {
	// 创建模拟的SSE响应
	sseData := `data: {"content": "chunk1"}

data: {"content": "chunk2"}

data: [DONE]

`
	// 创建测试服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte(sseData))
	}))
	defer server.Close()

	// 创建插件
	plugin := &RemoteNodePlugin{
		baseURL:         server.URL,
		httpClient:      &http.Client{},
		streamSupported: true,
		semaphore:       make(chan struct{}, 10),
	}

	// 测试流式执行
	req := &NodeExecutionRequest{
		SchemaVersion:  PipelinePluginSchemaVersion,
		Implementation: server.URL,
		Input:          &NodeInput{Content: "test"},
	}

	resp, err := plugin.executeStream(context.Background(), req)
	if err != nil {
		t.Fatalf("executeStream failed: %v", err)
	}

	if resp == nil || resp.Output == nil {
		t.Fatal("output is nil")
	}

	// 验证收集的内容（SSE解析会将所有content块合并）
	expected := "chunk1chunk2"
	if resp.Output.Content != expected {
		t.Errorf("expected %q, got %q", expected, resp.Output.Content)
	}
}

// TestManifestHashVerification 测试manifest哈希验证
func TestManifestHashVerification(t *testing.T) {
	// 创建一个简单的manifest
	manifest := `{"implementation": "test", "kind": "test", "version": "1.0.0"}`

	// 计算正确哈希
	hash := sha256.Sum256([]byte(manifest))
	correctHash := hex.EncodeToString(hash[:])

	// 创建测试服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".well-known/centag-node-plugin.json") {
			w.Write([]byte(manifest))
		}
	}))
	defer server.Close()

	// 测试正确哈希
	plugin := &RemoteNodePlugin{
		baseURL:    server.URL,
		httpClient: &http.Client{},
		hashConfig: ManifestHashConfig{
			Enabled: true,
			Hash:    correctHash,
		},
	}

	err := plugin.verifyManifestSignature(correctHash)
	if err != nil {
		t.Errorf("hash verification should pass with correct hash: %v", err)
	}

	// 测试错误哈希
	wrongHash := "wronghash123"
	err = plugin.verifyManifestSignature(wrongHash)
	if err == nil {
		t.Error("hash verification should fail with wrong hash")
	}
}

// TestValidateManifest 测试manifest校验
func TestValidateManifest(t *testing.T) {
	// 测试 nil descriptor
	err := validateManifest(nil)
	if err == nil {
		t.Error("Expected error for nil descriptor")
	}

	// 测试缺少 implementation
	err = validateManifest(&NodePluginDescriptor{})
	if err == nil {
		t.Error("Expected error for missing implementation")
	}

	// 测试缺少 kind
	err = validateManifest(&NodePluginDescriptor{
		Implementation: "test",
	})
	if err == nil {
		t.Error("Expected error for missing kind")
	}

	// 测试缺少 version
	err = validateManifest(&NodePluginDescriptor{
		Implementation: "test",
		Kind:           "remote.node",
	})
	if err == nil {
		t.Error("Expected error for missing version")
	}

	// 测试所有字段都有
	err = validateManifest(&NodePluginDescriptor{
		Implementation: "test",
		Kind:           "remote.node",
		Version:        "1.0.0",
	})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

// TestCircuitBreakerStateChange 测试熔断状态变化记录
func TestCircuitBreakerStateChange(t *testing.T) {
	// 创建插件
	plugin := &RemoteNodePlugin{
		baseURL:      "http://example.com",
		httpClient:   &http.Client{},
		semaphore:    make(chan struct{}, 10),
		healthStatus: "unknown",
	}

	// 初始状态
	if plugin.IsCircuitOpen() {
		t.Error("expected circuit to be closed initially")
	}

	// 模拟5次失败，触发熔断
	for i := 0; i < 5; i++ {
		plugin.recordFailure()
	}

	if !plugin.IsCircuitOpen() {
		t.Error("expected circuit to be open after 5 failures")
	}

	// 检查健康状态（熔断时应该是不健康）
	status, _ := plugin.GetHealthStatus()
	if status != "unhealthy" {
		t.Errorf("expected health status to be unhealthy, got %s", status)
	}

	// 模拟冷却时间已过，重置熔断
	plugin.setLastFailureForTest(time.Now().Add(-31 * time.Second))
	plugin.resetFailure()

	if plugin.IsCircuitOpen() {
		t.Error("expected circuit to be closed after cooldown")
	}
}

// TestHealthCheck 测试健康检查
func TestHealthCheck(t *testing.T) {
	// 创建测试服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			return
		}
		if strings.HasSuffix(r.URL.Path, ".well-known/centag-node-plugin.json") {
			w.Write([]byte(`{"implementation": "test", "kind": "test", "version": "1.0.0"}`))
			return
		}
	}))
	defer server.Close()

	plugin := &RemoteNodePlugin{
		baseURL:      server.URL,
		httpClient:   &http.Client{},
		semaphore:    make(chan struct{}, 10),
		healthStatus: "unknown",
	}

	// 执行健康检查
	plugin.performHealthCheck()

	// 检查健康状态
	status, _ := plugin.GetHealthStatus()
	if status != "healthy" {
		t.Errorf("expected healthy, got %s", status)
	}
}
