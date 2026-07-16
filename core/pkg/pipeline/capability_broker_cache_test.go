package pipeline

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// ========== Mock Implementations for Testing ==========

// mockCacheStrategyCapability 模拟缓存策略能力
type mockCacheStrategyCapability struct {
	strategyName string
	readResult   *CacheReadResult
	writeErr     error
	deleteErr    error
}

func (m *mockCacheStrategyCapability) Read(ctx context.Context, query string, threshold float32, topK int) (*CacheReadResult, error) {
	if m.readResult != nil {
		return m.readResult, nil
	}
	return &CacheReadResult{Hit: false}, nil
}

func (m *mockCacheStrategyCapability) Write(ctx context.Context, key string, content string, ttl time.Duration) error {
	return m.writeErr
}

func (m *mockCacheStrategyCapability) Delete(ctx context.Context, key string) error {
	return m.deleteErr
}

func (m *mockCacheStrategyCapability) StrategyName() string {
	return m.strategyName
}

// mockCacheStrategyProvider 模拟缓存策略提供者
type mockCacheStrategyProvider struct {
	strategies map[string]CacheStrategyCapability
}

func (m *mockCacheStrategyProvider) GetStrategy(name string) (CacheStrategyCapability, error) {
	if s, ok := m.strategies[name]; ok {
		return s, nil
	}
	return nil, fmt.Errorf("strategy not found: %s", name)
}

// mockVectorCacheCapability 模拟向量缓存能力
type mockVectorCacheCapability struct{}

func (m *mockVectorCacheCapability) Search(ctx context.Context, vector []float32, topK int, threshold float32) ([]VectorSearchResult, error) {
	return []VectorSearchResult{}, nil
}

func (m *mockVectorCacheCapability) Insert(ctx context.Context, id string, vector []float32, metadata map[string]interface{}) error {
	return nil
}

func (m *mockVectorCacheCapability) Delete(ctx context.Context, id string) error {
	return nil
}

// mockVectorCacheProvider 模拟向量缓存提供者
type mockVectorCacheProvider struct{}

func (m *mockVectorCacheProvider) GetVectorCache(namespace string) (VectorCacheCapability, error) {
	return &mockVectorCacheCapability{}, nil
}

// mockEmbeddingCapability 模拟嵌入服务能力
type mockEmbeddingCapability struct {
	dimension int
}

func (m *mockEmbeddingCapability) Embed(ctx context.Context, text string) ([]float32, error) {
	return make([]float32, m.dimension), nil
}

func (m *mockEmbeddingCapability) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	result := make([][]float32, len(texts))
	for i := range result {
		result[i] = make([]float32, m.dimension)
	}
	return result, nil
}

func (m *mockEmbeddingCapability) Dimension() int {
	return m.dimension
}

// mockEmbeddingProvider 模拟嵌入服务提供者
type mockEmbeddingProvider struct {
	dimension int
}

func (m *mockEmbeddingProvider) GetEmbeddingService(model string) (EmbeddingCapability, error) {
	return &mockEmbeddingCapability{dimension: m.dimension}, nil
}

// ========== Tests ==========

func TestCapabilityBrokerGetCacheStrategy(t *testing.T) {
	// 创建带缓存策略的 CapabilityBroker
	broker := NewCapabilityBroker(nil, nil, nil, HTTPConfig{})

	// 设置缓存策略提供者
	provider := &mockCacheStrategyProvider{
		strategies: map[string]CacheStrategyCapability{
			"exact": &mockCacheStrategyCapability{strategyName: "exact"},
		},
	}
	broker.SetCacheStrategyProvider(provider)

	// 测试获取缓存策略
	ctx := context.Background()
	permissions := []string{"cache.read", "cache.write"}

	strategy, err := broker.GetCacheStrategy(ctx, "exact", permissions)
	if err != nil {
		t.Fatalf("failed to get cache strategy: %v", err)
	}

	if strategy.StrategyName() != "exact" {
		t.Errorf("expected strategy name 'exact', got '%s'", strategy.StrategyName())
	}
}

func TestCapabilityBrokerGetCacheStrategyNoPermission(t *testing.T) {
	broker := NewCapabilityBroker(nil, nil, nil, HTTPConfig{})

	ctx := context.Background()
	permissions := []string{} // 无权限

	_, err := broker.GetCacheStrategy(ctx, "exact", permissions)
	if err == nil {
		t.Error("expected permission denied error")
	}
}

func TestCapabilityBrokerGetVectorCache(t *testing.T) {
	broker := NewCapabilityBroker(nil, nil, nil, HTTPConfig{})
	broker.SetVectorCacheProvider(&mockVectorCacheProvider{})

	ctx := context.Background()
	permissions := []string{"vector.read", "vector.write"}

	vc, err := broker.GetVectorCache(ctx, permissions)
	if err != nil {
		t.Fatalf("failed to get vector cache: %v", err)
	}

	if vc == nil {
		t.Error("expected non-nil vector cache")
	}
}

func TestCapabilityBrokerGetEmbeddingService(t *testing.T) {
	broker := NewCapabilityBroker(nil, nil, nil, HTTPConfig{})
	broker.SetEmbeddingProvider(&mockEmbeddingProvider{dimension: 1536})

	ctx := context.Background()
	permissions := []string{"embedding.generate"}

	emb, err := broker.GetEmbeddingService(ctx, permissions)
	if err != nil {
		t.Fatalf("failed to get embedding service: %v", err)
	}

	if emb.Dimension() != 1536 {
		t.Errorf("expected dimension 1536, got %d", emb.Dimension())
	}
}

func TestNewCapabilityBrokerWithCache(t *testing.T) {
	cacheProvider := &mockCacheStrategyProvider{
		strategies: map[string]CacheStrategyCapability{
			"exact": &mockCacheStrategyCapability{strategyName: "exact"},
		},
	}

	broker := NewCapabilityBrokerWithCache(
		nil,                       // storageProvider
		nil,                       // memoryProvider
		nil,                       // secretsProvider
		HTTPConfig{},              // httpConfig
		cacheProvider,             // cacheStrategyProvider
		&mockVectorCacheProvider{}, // vectorCacheProvider
		&mockEmbeddingProvider{dimension: 768}, // embeddingProvider
	)

	if broker.cacheStrategyProvider == nil {
		t.Error("expected cacheStrategyProvider to be set")
	}
	if broker.vectorCacheProvider == nil {
		t.Error("expected vectorCacheProvider to be set")
	}
	if broker.embeddingProvider == nil {
		t.Error("expected embeddingProvider to be set")
	}
}

func TestCacheStrategyCapabilityInterface(t *testing.T) {
	// 验证接口实现
	var _ CacheStrategyCapability = &mockCacheStrategyCapability{}
}

func TestVectorCacheCapabilityInterface(t *testing.T) {
	// 验证接口实现
	var _ VectorCacheCapability = &mockVectorCacheCapability{}
}

func TestEmbeddingCapabilityInterface(t *testing.T) {
	// 验证接口实现
	var _ EmbeddingCapability = &mockEmbeddingCapability{}
}

func TestCacheReadResult(t *testing.T) {
	result := &CacheReadResult{
		Hit:     true,
		Content: "test response",
		Key:     "test-key",
		Score:   0.95,
	}

	if !result.Hit {
		t.Error("expected Hit to be true")
	}
	if result.Content != "test response" {
		t.Errorf("unexpected content: %s", result.Content)
	}
	if result.Score != 0.95 {
		t.Errorf("unexpected score: %f", result.Score)
	}
}

func TestVectorSearchResult(t *testing.T) {
	result := VectorSearchResult{
		ID:       "vec-1",
		Score:    0.92,
		Metadata: map[string]interface{}{"request": "test"},
	}

	if result.ID != "vec-1" {
		t.Errorf("unexpected ID: %s", result.ID)
	}
	if result.Score != 0.92 {
		t.Errorf("unexpected score: %f", result.Score)
	}
}

// ========== CacheNode Strategy Plugin Tests ==========

func TestCacheNodeSetStrategyPlugin(t *testing.T) {
	node, err := NewCacheNode(NodeConfig{
		CustomConfig: map[string]interface{}{
			"operation": "read",
			"strategy":  "semantic",
		},
	})
	if err != nil {
		t.Fatalf("failed to create cache node: %v", err)
	}

	cacheNode := node.(*CacheNode)

	// 设置策略插件
	plugin := &mockCacheStrategyCapability{strategyName: "semantic"}
	cacheNode.SetStrategyPlugin(plugin)

	// 验证设置成功
	if !cacheNode.IsUsingStrategyPlugin() {
		t.Error("expected to be using strategy plugin")
	}
	if cacheNode.GetStrategyPlugin().StrategyName() != "semantic" {
		t.Errorf("unexpected strategy name: %s", cacheNode.GetStrategyPlugin().StrategyName())
	}
}

func TestCacheNodeStrategyPluginConfig(t *testing.T) {
	node, err := NewCacheNode(NodeConfig{
		CustomConfig: map[string]interface{}{
			"operation":           "read",
			"strategy":            "semantic",
			"use_strategy_plugin": true,
			"semantic_threshold":  0.9,
			"semantic_top_k":      10.0,
		},
	})
	if err != nil {
		t.Fatalf("failed to create cache node: %v", err)
	}

	cacheNode := node.(*CacheNode)

	if !cacheNode.useStrategyPlugin {
		t.Error("expected useStrategyPlugin to be true")
	}
	if cacheNode.semanticThreshold != 0.9 {
		t.Errorf("expected semanticThreshold 0.9, got %f", cacheNode.semanticThreshold)
	}
	if cacheNode.semanticTopK != 10 {
		t.Errorf("expected semanticTopK 10, got %d", cacheNode.semanticTopK)
	}
}

func TestCacheNodeExecuteWithStrategyPlugin(t *testing.T) {
	node, err := NewCacheNode(NodeConfig{
		CustomConfig: map[string]interface{}{
			"operation": "read",
			"strategy":  "exact",
		},
	})
	if err != nil {
		t.Fatalf("failed to create cache node: %v", err)
	}

	cacheNode := node.(*CacheNode)

	// 设置策略插件，模拟命中
	plugin := &mockCacheStrategyCapability{
		strategyName: "exact",
		readResult: &CacheReadResult{
			Hit:     true,
			Content: "cached response",
			Key:     "test-key",
			Score:   1.0,
		},
	}
	cacheNode.SetStrategyPlugin(plugin)

	// 执行读取操作
	ctx := context.Background()
	input := &NodeInput{
		Content: "test query",
	}

	output, err := cacheNode.Execute(ctx, input)
	if err != nil {
		t.Fatalf("failed to execute: %v", err)
	}

	// 验证结果
	if output.Content != "cached response" {
		t.Errorf("unexpected content: %s", output.Content)
	}
	if hit, ok := output.Metadata["cache_hit"].(bool); !ok || !hit {
		t.Error("expected cache_hit to be true")
	}
}
