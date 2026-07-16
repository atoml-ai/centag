package backend

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// BenchmarkBackendSelection 测试后端选择性能
func BenchmarkBackendSelection(b *testing.B) {
	backends := []string{"openai-1", "ollama-1", "anthropic-1", "openai-2"}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// 模拟轮询选择
		_ = backends[i%len(backends)]
	}
}

// BenchmarkHealthCheckStatus 测试健康检查状态查询性能
func BenchmarkHealthCheckStatus(b *testing.B) {
	statuses := map[string]HealthStatus{
		"openai-1":   {Status: "healthy", LastCheck: time.Now()},
		"ollama-1":   {Status: "healthy", LastCheck: time.Now()},
		"anthropic-1": {Status: "unhealthy", LastCheck: time.Now()},
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		backendID := fmt.Sprintf("backend-%d", i%3)
		_ = statuses[backendID]
	}
}

// BenchmarkRequestPreparation 测试请求准备性能
func BenchmarkRequestPreparation(b *testing.B) {
	ctx := context.Background()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		_ = reqCtx
		cancel()
	}
}

// BenchmarkResponseTimeCalculation 测试响应时间计算性能
func BenchmarkResponseTimeCalculation(b *testing.B) {
	start := time.Now().Add(-time.Duration(100) * time.Millisecond)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		elapsed := time.Since(start)
		_ = elapsed.Milliseconds()
	}
}

// BenchmarkBackendIDLookup 测试后端 ID 查找性能
func BenchmarkBackendIDLookup(b *testing.B) {
	backendMap := map[string]*Backend{
		"openai-1":    {ID: "openai-1", Name: "OpenAI GPT-4"},
		"ollama-1":    {ID: "ollama-1", Name: "Ollama Local"},
		"anthropic-1": {ID: "anthropic-1", Name: "Anthropic Claude"},
	}
	
	ids := []string{"openai-1", "ollama-1", "anthropic-1"}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id := ids[i%len(ids)]
		_ = backendMap[id]
	}
}

// Backend 模拟后端结构
type Backend struct {
	ID       string
	Name     string
	BaseURL  string
	APIKey   string
	Models   []string
	Enabled  bool
	Healthy  bool
	LastUsed time.Time
}

// HealthStatus 健康状态
type HealthStatus struct {
	Status    string
	LastCheck time.Time
	Latency   time.Duration
}
