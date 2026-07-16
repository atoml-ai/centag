package pipeline

import (
	"context"
	"testing"
	"time"

	"centag/core/internal/cache"
	"centag/core/internal/cache/evaluation/plugin"
	"centag/core/pkg/storage"
	"centag/core/pkg/processor"
)

// mockCacheManager 模拟缓存管理器用于测试
type mockCacheManager struct {
	data map[string]*cache.CacheEntry
}

func newMockCacheManager() *mockCacheManager {
	return &mockCacheManager{
		data: make(map[string]*cache.CacheEntry),
	}
}

func (m *mockCacheManager) Get(ctx context.Context, key string) (*cache.CacheEntry, error) {
	if entry, ok := m.data[key]; ok {
		return entry, nil
	}
	return nil, nil
}

func (m *mockCacheManager) Set(ctx context.Context, key string, value *cache.CacheEntry, ttl time.Duration) error {
	m.data[key] = value
	return nil
}

func (m *mockCacheManager) Delete(ctx context.Context, key string) error {
	delete(m.data, key)
	return nil
}

func (m *mockCacheManager) Clear(ctx context.Context) error {
	m.data = make(map[string]*cache.CacheEntry)
	return nil
}

func (m *mockCacheManager) Stats() *cache.CacheStats {
	return &cache.CacheStats{
		TotalEntries: int64(len(m.data)),
	}
}

func (m *mockCacheManager) Hit()   {}
func (m *mockCacheManager) Miss() {}

// 添加缺失的方法以完整实现 cache.CacheManager 接口

func (m *mockCacheManager) GetQASplitter() *processor.QASplitter {
	return nil // 测试不需要 QA 拆分功能
}

func (m *mockCacheManager) GetSemanticCache() *cache.SemanticCacheImpl {
	return nil // 测试不需要语义缓存
}

func (m *mockCacheManager) GetSemanticCacheStore() storage.KVStore {
	return nil // 测试不需要语义缓存存储
}

func (m *mockCacheManager) ShouldEvaluateCache() bool {
	return false // 测试默认不启用评估
}

func (m *mockCacheManager) EvaluateCacheEntry(ctx context.Context, question, answer string, historyMessages []plugin.Message) (*cache.EvaluationResult, error) {
	// 测试默认返回评估通过
	return &cache.EvaluationResult{
		Score:       80.0,
		ShouldCache: true,
		Labels:      []string{"test"},
	}, nil
}

// TestCachePipelineWithHit 测试缓存命中时的流水线行为
func TestCachePipelineWithHit(t *testing.T) {
	// 创建模拟缓存管理器并预置缓存数据
	// 注意：simpleHash 对于短内容（<=16字符）会返回整个内容
	cacheKey := "GLM-4-flash:python的用途"
	cacheMgr := newMockCacheManager()
	cacheMgr.data[cacheKey] = &cache.CacheEntry{
		Key:      cacheKey,
		Request:  "python的用途",
		Response: "Python是一种高级编程语言，广泛应用于Web开发、数据分析、人工智能等领域。",
		Metadata: map[string]interface{}{
			"model": "GLM-4-flash",
		},
		Timestamp: time.Now(),
		ExpiresAt: time.Now().Add(time.Hour),
	}

	// 创建缓存读取节点
	cacheReadNode, err := NewCacheNode(NodeConfig{
		CustomConfig: map[string]interface{}{
			"operation":    "read",
			"strategy":     "exact",
			"storage_type": "memory",
			"key_template": "{{model}}:{{hash}}",
		},
	})
	if err != nil {
		t.Fatalf("Failed to create cache read node: %v", err)
	}

	// 注入缓存管理器
	if cn, ok := cacheReadNode.(*CacheNode); ok {
		cn.SetCacheManager(cacheMgr)
	}

	// 创建输入
	input := &NodeInput{
		Content: "python的用途",
		Metadata: map[string]interface{}{
			"model": "GLM-4-flash",
		},
	}

	// 执行缓存读取
	ctx := context.Background()
	output, err := cacheReadNode.Execute(ctx, input)
	if err != nil {
		t.Fatalf("Cache read execution failed: %v", err)
	}

	// 验证缓存命中
	cacheHit, ok := output.Metadata["cache_hit"].(bool)
	if !ok || !cacheHit {
		t.Errorf("Expected cache hit, but got cache miss")
	}

	// 验证返回了缓存内容
	if output.Content == "" {
		t.Errorf("Expected cached content, but got empty content")
	}

	t.Logf("Cache hit test passed: cache_hit=%v, content_length=%d", cacheHit, len(output.Content))
}

// TestCachePipelineWithMiss 测试缓存未命中时的流水线行为
func TestCachePipelineWithMiss(t *testing.T) {
	// 创建空的模拟缓存管理器
	cacheMgr := newMockCacheManager()

	// 创建缓存读取节点
	cacheReadNode, err := NewCacheNode(NodeConfig{
		CustomConfig: map[string]interface{}{
			"operation":    "read",
			"strategy":     "exact",
			"storage_type": "memory",
			"key_template": "{{model}}:{{hash}}",
		},
	})
	if err != nil {
		t.Fatalf("Failed to create cache read node: %v", err)
	}

	// 注入缓存管理器
	if cn, ok := cacheReadNode.(*CacheNode); ok {
		cn.SetCacheManager(cacheMgr)
	}

	// 创建输入
	input := &NodeInput{
		Content: "这是一个新的问题",
		Metadata: map[string]interface{}{
			"model": "GLM-4-flash",
		},
	}

	// 执行缓存读取
	ctx := context.Background()
	output, err := cacheReadNode.Execute(ctx, input)
	if err != nil {
		t.Fatalf("Cache read execution failed: %v", err)
	}

	// 验证缓存未命中
	cacheHit, ok := output.Metadata["cache_hit"].(bool)
	if !ok || cacheHit {
		t.Errorf("Expected cache miss, but got cache hit")
	}

	// 验证内容为空
	if output.Content != "" {
		t.Errorf("Expected empty content on cache miss, but got: %s", output.Content)
	}

	t.Logf("Cache miss test passed: cache_hit=%v", cacheHit)
}

// TestCacheWriteNode 测试缓存写入节点
func TestCacheWriteNode(t *testing.T) {
	// 创建模拟缓存管理器
	cacheMgr := newMockCacheManager()

	// 创建缓存写入节点
	cacheWriteNode, err := NewCacheNode(NodeConfig{
		CustomConfig: map[string]interface{}{
			"operation":    "write",
			"strategy":     "exact",
			"storage_type": "memory",
			"ttl":          float64(3600),
			"key_template": "{{model}}:{{hash}}",
		},
	})
	if err != nil {
		t.Fatalf("Failed to create cache write node: %v", err)
	}

	// 注入缓存管理器
	if cn, ok := cacheWriteNode.(*CacheNode); ok {
		cn.SetCacheManager(cacheMgr)
	}

	// 创建执行上下文，提供原始用户输入
	pipeline := &AgentPatternPipeline{ID: "test-pipeline"}
	execCtx := NewExecutionContext(pipeline)
	execCtx.SetVariable("input", "python的用途") // 设置原始用户输入

	// 创建输入（模拟 generator 节点的输出）
	input := &NodeInput{
		Content: "Python是一种高级编程语言，广泛应用于Web开发、数据分析、人工智能等领域。",
		Metadata: map[string]interface{}{
			"model":   "GLM-4-flash",
			"backend": "bigmodel",
		},
		Messages: []Message{
			{Role: "user", Content: "python的用途"},
			{Role: "assistant", Content: "Python是一种高级编程语言，广泛应用于Web开发、数据分析、人工智能等领域。"},
		},
	}

	// 将 execCtx 注入 context
	ctx := context.Background()
	ctx = context.WithValue(ctx, executionContextKey{}, execCtx)

	// 标记测试模式，跳过 generator 执行检查（executeWrite 会检查 generator 是否执行）
	execCtx.SetVariable("test_mode", true)

	// 执行缓存写入
	output, err := cacheWriteNode.Execute(ctx, input)
	if err != nil {
		t.Fatalf("Cache write execution failed: %v", err)
	}

	// 验证写入成功
	written, ok := output.Metadata["written"].(bool)
	if !ok || !written {
		t.Errorf("Expected cache written, but got not written")
	}

	// 验证缓存中确实有数据（使用正确的缓存键）
	cacheKey := "GLM-4-flash:python的用途"
	if _, exists := cacheMgr.data[cacheKey]; !exists {
		t.Errorf("Expected cache entry '%s' to exist, but it doesn't. Keys: %v", cacheKey, getMapKeys(cacheMgr.data))
	}

	t.Logf("Cache write test passed: written=%v, cache_size=%d", written, len(cacheMgr.data))
}

// getMapKeys 辅助函数：获取 map 的所有键
func getMapKeys(m map[string]*cache.CacheEntry) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// TestTokenUsageNodeWithGeneratorOutput 测试 TokenUsageNode 从 generator 输出获取数据
func TestTokenUsageNodeWithGeneratorOutput(t *testing.T) {
	// 创建执行上下文
	pipeline := &AgentPatternPipeline{ID: "test-pipeline"}
	execCtx := NewExecutionContext(pipeline)

	// 模拟 generator 节点的输出（兼容旧 alias ID）
	execCtx.SetResult("generator", &NodeOutput{
		Content: "Python是一种高级编程语言",
		Metadata: map[string]interface{}{
			"model":         "GLM-4-flash",
			"tokens":        424,
			"prompt_tokens": 64,
		},
	})

	// 创建 TokenUsageNode
	tokenUsageNode, err := NewTokenUsageNode(NodeConfig{
		CustomConfig: map[string]interface{}{
			"operation":    "record",
			"storage_type": "memory",
			"record_fields": []interface{}{
				"prompt_tokens",
				"completion_tokens",
				"total_tokens",
				"model",
				"backend_id",
			},
		},
	})
	if err != nil {
		t.Fatalf("Failed to create token usage node: %v", err)
	}

	// 创建输入
	input := &NodeInput{
		Content: "python的用途",
		Metadata: map[string]interface{}{
			"backend_id": "bigmodel",
		},
	}

	// 将 execCtx 注入 context
	ctx := context.Background()
	ctx = context.WithValue(ctx, executionContextKey{}, execCtx)

	// 执行记录
	output, err := tokenUsageNode.Execute(ctx, input)
	if err != nil {
		t.Fatalf("Token usage execution failed: %v", err)
	}

	// 验证记录的数据
	usageRecord, ok := output.Metadata["usage_record"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected usage_record in metadata")
	}

	// 验证 model 字段
	model, ok := usageRecord["model"].(string)
	if !ok || model == "" {
		t.Errorf("Expected model to be set, but got: %v", usageRecord["model"])
	}

	// 验证 total_tokens 字段
	totalTokens, ok := usageRecord["total_tokens"].(int)
	if !ok || totalTokens == 0 {
		t.Errorf("Expected total_tokens to be set, but got: %v", usageRecord["total_tokens"])
	}

	t.Logf("Token usage test passed: model=%s, total_tokens=%d", model, totalTokens)
}

func TestTokenUsageNodeWithRealNodeID(t *testing.T) {
	const genID = "node-1781164287802"
	pipeline := &AgentPatternPipeline{ID: "test-pipeline"}
	execCtx := NewExecutionContext(pipeline)
	execCtx.SetResult(genID, &NodeOutput{
		Content: "回答内容",
		Metadata: map[string]interface{}{
			"model":         "deepseek-v4-flash",
			"backend_id":    "deepseek",
			"tokens":        int64(472),
			"prompt_tokens": float64(320),
		},
	})

	tokenUsageNode, err := NewTokenUsageNode(NodeConfig{
		CustomConfig: map[string]interface{}{
			"operation":    "record",
			"storage_type": "sqlite",
		},
	})
	if err != nil {
		t.Fatalf("NewTokenUsageNode: %v", err)
	}

	input := &NodeInput{
		Content: "用户问题",
		Metadata: map[string]interface{}{
			genID: map[string]interface{}{
				"metadata": map[string]interface{}{
					"model":         "deepseek-v4-flash",
					"backend_id":    "deepseek",
					"tokens":        int64(472),
					"prompt_tokens": float64(320),
				},
			},
		},
		UpstreamResults: map[string]*NodeOutput{
			genID: {
				Content: "回答内容",
				Metadata: map[string]interface{}{
					"model":         "deepseek-v4-flash",
					"backend_id":    "deepseek",
					"tokens":        int64(472),
					"prompt_tokens": float64(320),
				},
			},
		},
	}

	ctx := context.Background()
	ctx = context.WithValue(ctx, executionContextKey{}, execCtx)

	output, err := tokenUsageNode.Execute(ctx, input)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	usageRecord, ok := output.Metadata["usage_record"].(map[string]interface{})
	if !ok {
		t.Fatal("missing usage_record")
	}
	if got := tokenRecordString(usageRecord["model"]); got != "deepseek-v4-flash" {
		t.Fatalf("model = %q, want deepseek-v4-flash", got)
	}
	if got := tokenRecordInt(usageRecord["total_tokens"]); got != 472 {
		t.Fatalf("total_tokens = %d, want 472", got)
	}
	if got := tokenRecordInt(usageRecord["prompt_tokens"]); got != 320 {
		t.Fatalf("prompt_tokens = %d, want 320", got)
	}
	if got := tokenRecordString(usageRecord["backend_id"]); got != "deepseek" {
		t.Fatalf("backend_id = %q, want deepseek", got)
	}
}

// TestCacheKeyTemplateResolution 测试缓存键模板解析
func TestCacheKeyTemplateResolution(t *testing.T) {
	// 创建缓存节点
	cacheNode, err := NewCacheNode(NodeConfig{
		CustomConfig: map[string]interface{}{
			"operation":    "read",
			"strategy":     "exact",
			"storage_type": "memory",
			"key_template": "{{model}}:{{hash}}",
		},
	})
	if err != nil {
		t.Fatalf("Failed to create cache node: %v", err)
	}

	cn := cacheNode.(*CacheNode)

	// 测试带有 model 的输入
	input := &NodeInput{
		Content: "python的用途",
		Metadata: map[string]interface{}{
			"model": "GLM-4-flash",
		},
	}

	cacheKey := cn.buildCacheKey(input)

	// 验证 {{model}} 被正确替换
	if cacheKey == "{{model}}:python的用途" {
		t.Errorf("Cache key template not resolved, got: %s", cacheKey)
	}

	// 验证包含模型名称（注意：simpleHash 对于短内容返回整个内容）
	expectedKey := "GLM-4-flash:python的用途"
	if cacheKey != expectedKey {
		t.Errorf("Expected cache key '%s', got: %s", expectedKey, cacheKey)
	}

	t.Logf("Cache key template resolution test passed: key=%s", cacheKey)
}

// TestCacheKeyTemplateWithMissingModel 测试缓存键模板缺少 model 时的处理
func TestCacheKeyTemplateWithMissingModel(t *testing.T) {
	// 创建缓存节点
	cacheNode, err := NewCacheNode(NodeConfig{
		CustomConfig: map[string]interface{}{
			"operation":    "read",
			"strategy":     "exact",
			"storage_type": "memory",
			"key_template": "{{model}}:{{hash}}",
		},
	})
	if err != nil {
		t.Fatalf("Failed to create cache node: %v", err)
	}

	cn := cacheNode.(*CacheNode)

	// 测试没有 model 的输入
	input := &NodeInput{
		Content: "python的用途",
	}

	cacheKey := cn.buildCacheKey(input)

	// 验证 {{model}} 被替换为 default
	if cacheKey == "{{model}}:python的用途" {
		t.Errorf("Cache key template not resolved when model is missing, got: %s", cacheKey)
	}

	// 验证使用 default 作为模型名称
	expectedKey := "default:python的用途"
	if cacheKey != expectedKey {
		t.Errorf("Expected cache key '%s', got: %s", expectedKey, cacheKey)
	}

	t.Logf("Cache key template with missing model test passed: key=%s", cacheKey)
}

// TestCacheNodeSemanticStrategyConfig 测试缓存节点语义策略配置读取
func TestCacheNodeSemanticStrategyConfig(t *testing.T) {
	config := NodeConfig{
		CustomConfig: map[string]interface{}{
			"operation":           "read",
			"strategy":            "semantic",
			"storage_type":        "memory",
			"read_storage_name":   "default-vector",
			"vector_storage_name": "default-vector",
			"embedding_model":     "text-embedding-3-small",
			"semantic_threshold":  float64(0.90),
			"semantic_top_k":      float64(10),
		},
	}

	node, err := NewCacheNode(config)
	if err != nil {
		t.Fatalf("Failed to create cache node: %v", err)
	}

	cn := node.(*CacheNode)

	if cn.Strategy != "semantic" {
		t.Errorf("expected strategy 'semantic', got '%s'", cn.Strategy)
	}

	if cn.ReadStorageName != "default-vector" {
		t.Errorf("expected read_storage_name 'default-vector', got '%s'", cn.ReadStorageName)
	}

	if cn.semanticThreshold != 0.90 {
		t.Errorf("expected semantic_threshold 0.90, got %f", cn.semanticThreshold)
	}

	if cn.semanticTopK != 10 {
		t.Errorf("expected semantic_top_k 10, got %d", cn.semanticTopK)
	}

	t.Logf("CacheNode semantic strategy config test passed")
}

// TestCacheNodeHybridStrategyConfig 测试缓存节点混合策略配置读取
func TestCacheNodeHybridStrategyConfig(t *testing.T) {
	config := NodeConfig{
		CustomConfig: map[string]interface{}{
			"operation":           "write",
			"strategy":            "hybrid",
			"storage_type":        "memory",
			"write_storage_name":  "default-vector",
			"vector_storage_name": "default-vector",
			"embedding_model":     "text-embedding-ada-002",
			"semantic_threshold":  float64(0.80),
			"semantic_top_k":      float64(3),
			"ttl":                 float64(7200),
		},
	}

	node, err := NewCacheNode(config)
	if err != nil {
		t.Fatalf("Failed to create cache node: %v", err)
	}

	cn := node.(*CacheNode)

	if cn.Strategy != "hybrid" {
		t.Errorf("expected strategy 'hybrid', got '%s'", cn.Strategy)
	}

	if cn.WriteStorageName != "default-vector" {
		t.Errorf("expected write_storage_name 'default-vector', got '%s'", cn.WriteStorageName)
	}

	if cn.semanticThreshold != 0.80 {
		t.Errorf("expected semantic_threshold 0.80, got %f", cn.semanticThreshold)
	}

	if cn.semanticTopK != 3 {
		t.Errorf("expected semantic_top_k 3, got %d", cn.semanticTopK)
	}

	if cn.TTL != 7200 {
		t.Errorf("expected TTL 7200, got %d", cn.TTL)
	}

	t.Logf("CacheNode hybrid strategy config test passed")
}

// TestCacheNodeDefaultSemanticConfig 测试缓存节点语义配置默认值
func TestCacheNodeDefaultSemanticConfig(t *testing.T) {
	config := NodeConfig{
		CustomConfig: map[string]interface{}{
			"operation": "read",
			"strategy":  "semantic",
		},
	}

	node, err := NewCacheNode(config)
	if err != nil {
		t.Fatalf("Failed to create cache node: %v", err)
	}

	cn := node.(*CacheNode)

	// 验证默认值
	if cn.semanticThreshold != 0.85 {
		t.Errorf("expected default semantic_threshold 0.85, got %f", cn.semanticThreshold)
	}

	if cn.semanticTopK != 5 {
		t.Errorf("expected default semantic_top_k 5, got %d", cn.semanticTopK)
	}

	t.Logf("CacheNode default semantic config test passed")
}
