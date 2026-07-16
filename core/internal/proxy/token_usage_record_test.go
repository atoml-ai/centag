package proxy

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"centag/core/internal/auth"
	"centag/core/pkg/pipeline"
)

func TestExtractUserID_UsesAuthContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set(auth.CtxKeyUserID, int64(42))

	if got := extractUserID(c); got != "42" {
		t.Fatalf("extractUserID = %q, want 42", got)
	}
}

func TestPipelineDelegatedTokenUsage(t *testing.T) {
	delegated := &pipeline.PipelineOutput{
		ExecutionLog: &pipeline.ExecutionLog{
			NodeLogs: []pipeline.NodeExecutionLog{
				{NodeType: pipeline.NodeTypeGenerator, Success: true, InputTokens: 100, OutputTokens: 40},
				{NodeType: pipeline.NodeTypeTokenUsage, Success: true},
			},
		},
	}
	if !pipelineDelegatedTokenUsage(delegated) {
		t.Fatal("expected delegated token usage")
	}

	tokenUsageOnly := &pipeline.PipelineOutput{
		ExecutionLog: &pipeline.ExecutionLog{
			NodeLogs: []pipeline.NodeExecutionLog{
				{NodeType: pipeline.NodeTypeGenerator, Success: false},
				{NodeType: pipeline.NodeTypeTokenUsage, Success: true},
			},
		},
	}
	if pipelineDelegatedTokenUsage(tokenUsageOnly) {
		t.Fatal("expected proxy fallback when generator produced no tokens")
	}

	without := &pipeline.PipelineOutput{
		ExecutionLog: &pipeline.ExecutionLog{
			NodeLogs: []pipeline.NodeExecutionLog{
				{NodeType: pipeline.NodeTypeGenerator, Success: true, InputTokens: 10, OutputTokens: 5},
			},
		},
	}
	if pipelineDelegatedTokenUsage(without) {
		t.Fatal("expected no delegation without token_usage node")
	}
}

func TestTokenCountsFromOutput_PrefersNodeLogs(t *testing.T) {
	output := &pipeline.PipelineOutput{
		ExecutionLog: &pipeline.ExecutionLog{
			TotalTokens: 999,
			NodeLogs: []pipeline.NodeExecutionLog{
				{NodeType: pipeline.NodeTypeGenerator, Success: true, InputTokens: 100, OutputTokens: 40},
			},
		},
	}
	prompt, completion, total := tokenCountsFromOutput(output)
	if prompt != 100 || completion != 40 || total != 140 {
		t.Fatalf("counts = %d/%d/%d, want 100/40/140", prompt, completion, total)
	}
}