package cache

import (
	"fmt"
	"testing"
	"time"
)

// BenchmarkCacheKeyGeneration 测试缓存键生成性能
func BenchmarkCacheKeyGeneration(b *testing.B) {
	model := "gpt-4"
	messages := []Message{
		{Role: "user", Content: "Hello, how are you?"},
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = GenerateCacheKey(model, messages)
	}
}

// BenchmarkCacheKeyGenerationLargePayload 测试大负载缓存键生成性能
func BenchmarkCacheKeyGenerationLargePayload(b *testing.B) {
	model := "gpt-4"
	messages := []Message{
		{Role: "system", Content: "You are a helpful assistant."},
		{Role: "user", Content: "Can you explain the theory of relativity in detail?"},
		{Role: "assistant", Content: "The theory of relativity, developed by Albert Einstein, consists of two parts: special relativity and general relativity..."},
		{Role: "user", Content: "What about quantum mechanics?"},
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = GenerateCacheKey(model, messages)
	}
}

// BenchmarkSemanticHash 测试语义哈希性能
func BenchmarkSemanticHash(b *testing.B) {
	text := "What is the capital of France?"
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = SemanticHash(text)
	}
}

// BenchmarkTTLCalculation 测试 TTL 计算性能
func BenchmarkTTLCalculation(b *testing.B) {
	baseTTL := 3600 * time.Second
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// 模拟动态 TTL 计算
		ttl := baseTTL + time.Duration(i%100)*time.Second
		_ = ttl
	}
}

// BenchmarkCacheEntryCreation 测试缓存条目创建性能
func BenchmarkCacheEntryCreation(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		entry := &Entry{
			Key:       fmt.Sprintf("key-%d", i),
			Value:     []byte("test value"),
			CreatedAt: time.Now(),
			TTL:       3600 * time.Second,
		}
		_ = entry
	}
}

// Message 模拟消息结构
type Message struct {
	Role    string
	Content string
}

// GenerateCacheKey 生成缓存键
func GenerateCacheKey(model string, messages []Message) string {
	// 简化的缓存键生成
	key := model + ":"
	for _, msg := range messages {
		key += msg.Role + ":" + msg.Content + ":"
	}
	return key
}

// SemanticHash 生成语义哈希
func SemanticHash(text string) string {
	// 简化的语义哈希
	return fmt.Sprintf("hash-%d", len(text))
}

// Entry 缓存条目
type Entry struct {
	Key       string
	Value     []byte
	CreatedAt time.Time
	TTL       time.Duration
}
