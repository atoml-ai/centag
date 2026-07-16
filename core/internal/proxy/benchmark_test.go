package proxy

import (
	"context"
	"testing"
	"time"

	"centag/core/pkg/proxymode"
)

// BenchmarkProxyModeSelection 测试代理模式选择性能
func BenchmarkProxyModeSelection(b *testing.B) {
	mode := proxymode.ExecutionMode("smart-scheduling")
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = mode.IsValid()
	}
}

// BenchmarkBackendSelection 测试后端选择性能
func BenchmarkBackendSelection(b *testing.B) {
	backends := []string{"openai", "ollama", "anthropic"}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// 模拟轮询选择
		_ = backends[i%len(backends)]
	}
}

// BenchmarkRequestContextCreation 测试请求上下文创建性能
func BenchmarkRequestContextCreation(b *testing.B) {
	ctx := context.Background()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, cancel := context.WithTimeout(ctx, 30*time.Second)
		cancel()
	}
}

// BenchmarkProxyModeStringConversion 测试代理模式字符串转换性能
func BenchmarkProxyModeStringConversion(b *testing.B) {
	modes := []proxymode.ExecutionMode{
		proxymode.ExecutionMode("smart-scheduling"),
		proxymode.ExecutionMode("fallback"),
		proxymode.ExecutionMode("direct-backend"),
		proxymode.ExecutionMode("audit"),
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mode := modes[i%len(modes)]
		_ = mode.String()
	}
}

// BenchmarkProxyModeFromString 测试从字符串解析代理模式性能
func BenchmarkProxyModeFromString(b *testing.B) {
	modeStrings := []string{
		"smart-scheduling",
		"fallback",
		"direct-backend",
		"audit",
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = proxymode.FromString(modeStrings[i%len(modeStrings)])
	}
}
