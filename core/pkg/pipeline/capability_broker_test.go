package pipeline

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

// mockSecretsProvider 模拟密钥提供者
type mockSecretsProvider struct{}

func (m *mockSecretsProvider) ResolveSecret(ref string) (string, error) {
	if ref == "test-key" {
		return "test-value", nil
	}
	return "", nil
}

func TestDefaultCapabilityBrokerGetLLMClient(t *testing.T) {
	broker := NewCapabilityBroker(nil, nil, &mockSecretsProvider{}, HTTPConfig{})

	// 测试无权限
	_, err := broker.GetLLMClient(context.Background(), []string{})
	if err == nil {
		t.Error("Expected permission denied error")
	}

	// 测试有权限 - 现在 GetLLMClient 返回未实现错误
	_, err = broker.GetLLMClient(context.Background(), []string{"llm.call"})
	if err == nil {
		t.Error("Expected not implemented error")
	}
}

func TestDefaultCapabilityBrokerGetSecretsResolver(t *testing.T) {
	broker := NewCapabilityBroker(nil, nil, &mockSecretsProvider{}, HTTPConfig{})

	// 测试无权限
	_, err := broker.GetSecretsResolver(context.Background(), []string{})
	if err == nil {
		t.Error("Expected permission denied error")
	}

	// 测试有权限
	resolver, err := broker.GetSecretsResolver(context.Background(), []string{"secrets.read"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if resolver == nil {
		t.Error("Expected non-nil secrets resolver")
	}

	// 测试解析密钥
	value, err := resolver.Resolve("test-key")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if value != "test-value" {
		t.Errorf("Expected test-value, got %s", value)
	}
}

func TestDefaultCapabilityBrokerGetHTTPClient(t *testing.T) {
	broker := NewCapabilityBroker(nil, nil, nil, HTTPConfig{
		Allowlist: []string{"example.com"},
		Timeout:   10,
		MaxResponse: 1024,
		TLSVerify:  true,
	})

	// 测试无权限
	_, err := broker.GetHTTPClient(context.Background(), []string{})
	if err == nil {
		t.Error("Expected permission denied error")
	}

	// 测试有权限
	client, err := broker.GetHTTPClient(context.Background(), []string{"network.outbound"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if client == nil {
		t.Error("Expected non-nil HTTP client")
	}
}

func TestDefaultPermissionCheckerHasPermission(t *testing.T) {
	checker := &defaultPermissionChecker{}

	// 测试无权限
	if checker.HasPermission([]string{}, "llm.call") {
		t.Error("Expected false for empty permissions")
	}

	// 测试有权限
	if !checker.HasPermission([]string{"llm.call"}, "llm.call") {
		t.Error("Expected true for matching permission")
	}

	// 测试通配符
	if !checker.HasPermission([]string{"*"}, "llm.call") {
		t.Error("Expected true for wildcard permission")
	}
}

func TestExtractNamespace(t *testing.T) {
	// 测试无命名空间
	ns := extractNamespace([]string{"storage.read"}, "storage")
	if ns != "default" {
		t.Errorf("Expected default, got %s", ns)
	}

	// 测试有命名空间
	ns = extractNamespace([]string{"storage.read:my-namespace"}, "storage")
	if ns != "my-namespace" {
		t.Errorf("Expected my-namespace, got %s", ns)
	}

	// 测试多个权限，第一个匹配
	ns = extractNamespace([]string{"other", "storage.read:ns2", "storage.write:ns3"}, "storage")
	if ns != "ns2" {
		t.Errorf("Expected ns2, got %s", ns)
	}

	// 测试权限前缀不匹配
	ns = extractNamespace([]string{"other.read:ns"}, "storage")
	if ns != "default" {
		t.Errorf("Expected default for non-matching prefix, got %s", ns)
	}
}

func TestSplitPermission(t *testing.T) {
	// 测试无冒号
	parts := splitPermission("storage.read")
	if len(parts) != 1 || parts[0] != "storage.read" {
		t.Errorf("Expected [storage.read], got %v", parts)
	}

	// 测试有冒号
	parts = splitPermission("storage.read:my-namespace")
	if len(parts) != 2 || parts[0] != "storage.read" || parts[1] != "my-namespace" {
		t.Errorf("Expected [storage.read, my-namespace], got %v", parts)
	}

	// 测试多个冒号（只分割第一个）
	parts = splitPermission("a:b:c")
	if len(parts) != 2 || parts[0] != "a" || parts[1] != "b:c" {
		t.Errorf("Expected [a, b:c], got %v", parts)
	}
}

// mockStorageProvider 模拟存储提供者
type mockStorageProvider struct {
	storage *mockStorage
	err      error
}

func (m *mockStorageProvider) GetStorage(namespace string) (Storage, error) {
	return m.storage, m.err
}

// mockStorage 模拟存储
type mockStorage struct{}

func (m *mockStorage) Read(ctx context.Context, key string) ([]byte, error) {
	return []byte("data"), nil
}

func (m *mockStorage) Write(ctx context.Context, key string, value []byte) error {
	return nil
}

func (m *mockStorage) Delete(ctx context.Context, key string) error {
	return nil
}

func TestDefaultCapabilityBrokerGetStorage(t *testing.T) {
	// 测试无权限
	broker := NewCapabilityBroker(&mockStorageProvider{}, nil, nil, HTTPConfig{})
	_, err := broker.GetStorage(context.Background(), []string{})
	if err == nil {
		t.Error("Expected permission denied error")
	}

	// 测试有权限但无 storage provider
	broker2 := NewCapabilityBroker(nil, nil, nil, HTTPConfig{})
	_, err = broker2.GetStorage(context.Background(), []string{"storage.read"})
	if err == nil {
		t.Error("Expected storage provider not configured error")
	}

	// 测试有权限且有 storage provider
	broker3 := NewCapabilityBroker(&mockStorageProvider{storage: &mockStorage{}}, nil, nil, HTTPConfig{})
	storage, err := broker3.GetStorage(context.Background(), []string{"storage.read"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if storage == nil {
		t.Error("Expected non-nil storage")
	}
}

// mockMemoryProvider 模拟记忆提供者
type mockMemoryProvider struct {
	memory *mockMemory
	err    error
}

func (m *mockMemoryProvider) GetMemory(namespace string) (Memory, error) {
	return m.memory, m.err
}

// mockMemory 模拟记忆
type mockMemory struct{}

func (m *mockMemory) Read(ctx context.Context, key string) ([]byte, error) {
	return []byte("memory"), nil
}

func (m *mockMemory) Write(ctx context.Context, key string, value []byte) error {
	return nil
}

func (m *mockMemory) Search(ctx context.Context, query string, limit int) ([]MemoryResult, error) {
	return []MemoryResult{{Key: "key1", Score: 0.9, Data: []byte("data")}}, nil
}

func TestDefaultCapabilityBrokerGetMemory(t *testing.T) {
	// 测试无权限
	broker := NewCapabilityBroker(nil, &mockMemoryProvider{}, nil, HTTPConfig{})
	_, err := broker.GetMemory(context.Background(), []string{})
	if err == nil {
		t.Error("Expected permission denied error")
	}

	// 测试有权限但无 memory provider
	broker2 := NewCapabilityBroker(nil, nil, nil, HTTPConfig{})
	_, err = broker2.GetMemory(context.Background(), []string{"memory.read"})
	if err == nil {
		t.Error("Expected memory provider not configured error")
	}

	// 测试有权限且有 memory provider
	broker3 := NewCapabilityBroker(nil, &mockMemoryProvider{memory: &mockMemory{}}, nil, HTTPConfig{})
	memory, err := broker3.GetMemory(context.Background(), []string{"memory.read"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if memory == nil {
		t.Error("Expected non-nil memory")
	}
}

func TestControlledHTTPClient(t *testing.T) {
	// 测试 allowlist - 未授权主机
	client := &controlledHTTPClient{
		allowlist: []string{"allowed.com"},
		timeout:   1,
	}

	req, _ := http.NewRequest("GET", "http://evil.com/path", nil)
	_, err := client.Do(req)
	if err == nil {
		t.Error("Expected host not in allowlist error")
	}

	// 测试 allowlist - 允许所有
	client2 := &controlledHTTPClient{
		allowlist: []string{"*"},
		timeout:   1,
	}

	// 使用不存在的地址，应该因为连接失败而返回错误，而不是因为 allowlist
	req2, _ := http.NewRequest("GET", "http://nonexistent-test-domain-12345.com", nil)
	_, err2 := client2.Do(req2)
	// 不应该返回 allowlist 相关的错误
	if err2 != nil && containsString(err2.Error(), "not in allowlist") {
		t.Errorf("Should not get allowlist error with wildcard: %v", err2)
	}

	// 测试 allowlist - 特定主机匹配
	client3 := &controlledHTTPClient{
		allowlist: []string{"example.com"},
		timeout:   1,
	}

	req3, _ := http.NewRequest("GET", "http://example.com/path", nil)
	// 这个请求会因为无法连接而失败，但不应该因为 allowlist 失败
	_, err3 := client3.Do(req3)
	if err3 != nil && strings.Contains(err3.Error(), "not in allowlist") {
		t.Errorf("Should not get allowlist error for matching host: %v", err3)
	}
}

func TestSplitPermissionEdgeCases(t *testing.T) {
	// 空字符串
	parts := splitPermission("")
	if len(parts) != 1 || parts[0] != "" {
		t.Errorf("Expected [''], got %v", parts)
	}

	// 只有冒号
	parts = splitPermission(":")
	if len(parts) != 2 || parts[0] != "" || parts[1] != "" {
		t.Errorf("Expected ['', ''], got %v", parts)
	}
}
