package elasticsearch

import (
	"context"
	"os"
	"testing"
	"time"

	"centag/core/pkg/storage"
)

// TestPlugin_Initialization 测试插件初始化
func TestPlugin_Initialization(t *testing.T) {
	// 检查 ES 是否可用
	addresses := []string{"http://localhost:29200"}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	config := &Config{
		Addresses: addresses,
	}

	client, err := NewClient(config)
	if err != nil {
		t.Skipf("Skipping test: failed to create ES client: %v", err)
	}

	_, err = client.GetClusterHealth(ctx)
	if err != nil {
		t.Skipf("Skipping test: Elasticsearch is not available: %v", err)
	}

	configMap := map[string]interface{}{
		"addresses":        []string{"http://localhost:29200"},
		"exact_index":      "test_exact_index",
		"semantic_index":   "test_semantic_index",
		"vector_dimension": float64(128), // 测试用小维度
	}

	plugin, err := NewPlugin(configMap)
	if err != nil {
		t.Fatalf("Failed to create plugin: %v", err)
	}

	// 检查存储类型
	if plugin.StorageType() != StorageTypeElasticsearch {
		t.Errorf("Expected storage type %s, got %s", StorageTypeElasticsearch, plugin.StorageType())
	}

	t.Logf("Plugin created successfully")
}

// TestKVStore_BasicOperations 测试 KV 存储基本操作
func TestKVStore_BasicOperations(t *testing.T) {
	skipIfNoES(t)

	config := createTestConfig()
	plugin, err := NewPlugin(config)
	if err != nil {
		t.Fatalf("Failed to create plugin: %v", err)
	}

	kvStore, err := plugin.KVStore()
	if err != nil {
		t.Fatalf("Failed to get KV store: %v", err)
	}

	ctx := context.Background()
	testKey := "test_key_123"
	testValue := []byte("test_value_123")
	ttl := 10 * time.Minute

	// 测试 Set
	err = kvStore.Set(ctx, testKey, testValue, ttl)
	if err != nil {
		t.Fatalf("Failed to set key: %v", err)
	}

	// 测试 Get
	value, err := kvStore.Get(ctx, testKey)
	if err != nil {
		t.Fatalf("Failed to get key: %v", err)
	}

	retrievedValue, ok := value.([]byte)
	if !ok {
		t.Fatalf("Value is not []byte")
	}

	if string(retrievedValue) != string(testValue) {
		t.Errorf("Value mismatch: expected %s, got %s", string(testValue), string(retrievedValue))
	}

	// 测试 Exists
	exists, err := kvStore.Exists(ctx, testKey)
	if err != nil {
		t.Fatalf("Failed to check existence: %v", err)
	}

	if !exists {
		t.Error("Key should exist")
	}

	// 测试 Delete
	err = kvStore.Delete(ctx, testKey)
	if err != nil {
		t.Fatalf("Failed to delete key: %v", err)
	}

	// 清理
}

// TestVectorStore_BasicOperations 测试向量存储基本操作
func TestVectorStore_BasicOperations(t *testing.T) {
	skipIfNoES(t)

	dimension := 128
	config := map[string]interface{}{
		"addresses":        []string{"http://localhost:29200"},
		"exact_index":      "test_exact_index",
		"semantic_index":   "test_semantic_index",
		"vector_dimension": float64(dimension),
	}

	plugin, err := NewPlugin(config)
	if err != nil {
		t.Fatalf("Failed to create plugin: %v", err)
	}

	vectorStore, err := plugin.VectorStore()
	if err != nil {
		t.Fatalf("Failed to get vector store: %v", err)
	}

	ctx := context.Background()

	// 创建测试向量
	testVector := make([]float32, dimension)
	for i := 0; i < dimension; i++ {
		testVector[i] = 0.01 * float32(i)
	}

	vectors := []storage.Vector{
		{
			ID:     "test_vector_1",
			Vector: testVector,
			Metadata: map[string]interface{}{
				"model": "test-model",
			},
		},
	}

	// 测试 Insert
	err = vectorStore.Insert(ctx, vectors)
	if err != nil {
		t.Fatalf("Failed to insert vector: %v", err)
	}

	// 测试 Get
	retrievedVectors, err := vectorStore.Get(ctx, []string{"test_vector_1"})
	if err != nil {
		t.Fatalf("Failed to get vector: %v", err)
	}

	if len(retrievedVectors) != 1 {
		t.Fatalf("Expected 1 vector, got %d", len(retrievedVectors))
	}

	// 测试 Search
	searchResults, err := vectorStore.Search(ctx, testVector, 5, nil)
	if err != nil {
		t.Fatalf("Failed to search vectors: %v", err)
	}

	if len(searchResults) == 0 {
		t.Error("Expected at least 1 search result")
	}

	// 测试 Delete
	err = vectorStore.Delete(ctx, []string{"test_vector_1"})
	if err != nil {
		t.Fatalf("Failed to delete vector: %v", err)
	}

	// 清理
}

// 辅助函数

func createTestConfig() map[string]interface{} {
	return map[string]interface{}{
		"addresses":        []string{"http://localhost:29200"},
		"exact_index":      "test_exact_index",
		"semantic_index":   "test_semantic_index",
		"vector_dimension": float64(128),
	}
}

func skipIfNoES(t *testing.T) {
	// 支持通过环境变量强制运行测试
	// export ELASTICSEARCH_TEST_URL=http://localhost:29200
	address := getTestElasticsearchAddress()
	if address == "" {
		t.Skip("Skipping test: no Elasticsearch address configured. Set ELASTICSEARCH_TEST_URL env var or run ES on localhost:29200")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	config := &Config{
		Addresses: []string{address},
	}

	client, err := NewClient(config)
	if err != nil {
		t.Skipf("Skipping test: failed to create ES client: %v", err)
	}

	_, err = client.GetClusterHealth(ctx)
	if err != nil {
		t.Skipf("Skipping test: Elasticsearch is not available at %s: %v", address, err)
	}
}

// getTestElasticsearchAddress 从环境变量或默认地址获取 ES 地址
func getTestElasticsearchAddress() string {
	// 优先使用环境变量
	if addr := os.Getenv("ELASTICSEARCH_TEST_URL"); addr != "" {
		return addr
	}
	// 默认地址
	return "http://localhost:29200"
}
