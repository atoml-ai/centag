package pipeline

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// BenchmarkNodeExecution 测试节点执行性能
func BenchmarkNodeExecution(b *testing.B) {
	ctx := context.Background()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		_ = reqCtx
		cancel()
	}
}

// BenchmarkNodeConfigAccess 测试节点配置访问性能
func BenchmarkNodeConfigAccess(b *testing.B) {
	config := NodeConfig{
		Backend:        "openai",
		Model:          "gpt-4",
		PromptTemplate: "Hello {{name}}",
		Temperature:    floatPtr(0.7),
		MaxTokens:      intPtr(100),
		CustomConfig: map[string]interface{}{
			"timeout": 30,
			"retry":   3,
		},
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = config.Backend
		_ = config.Model
		_ = config.CustomConfig["timeout"]
	}
}

// BenchmarkNodeInputCreation 测试节点输入创建性能
func BenchmarkNodeInputCreation(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		input := &NodeInput{
			Content: fmt.Sprintf("Test input %d", i),
			Metadata: map[string]interface{}{
				"index": i,
			},
		}
		_ = input
	}
}

// BenchmarkNodeOutputCreation 测试节点输出创建性能
func BenchmarkNodeOutputCreation(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		output := &NodeOutput{
			Content: fmt.Sprintf("Test output %d", i),
			Metadata: map[string]interface{}{
				"tokens": i * 10,
			},
		}
		_ = output
	}
}

// BenchmarkPipelineContextCreation 测试流水线上下文创建性能
func BenchmarkPipelineContextCreation(b *testing.B) {
	ctx := context.Background()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pipelineCtx := &PipelineContext{
			TraceID:   fmt.Sprintf("trace-%d", i),
			RequestID: fmt.Sprintf("req-%d", i),
			StartTime: time.Now(),
		}
		_ = pipelineCtx
		_ = ctx
	}
}

// BenchmarkNodeIDGeneration 测试节点 ID 生成性能
func BenchmarkNodeIDGeneration(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		nodeID := fmt.Sprintf("node-%d", i)
		_ = nodeID
	}
}

// PipelineContext 流水线上下文
type PipelineContext struct {
	TraceID   string
	RequestID string
	StartTime time.Time
}

func floatPtr(f float64) *float64 {
	return &f
}

func intPtr(i int) *int {
	return &i
}
