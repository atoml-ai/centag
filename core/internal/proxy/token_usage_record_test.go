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

func TestTokenCountsFromSSEContent_ParsesUsageChunk(t *testing.T) {
	sse := "data: {\"choices\":[{\"delta\":{\"content\":\"hi\"}}]}\n\n" +
		"data: {\"usage\":{\"prompt_tokens\":12,\"completion_tokens\":3,\"total_tokens\":15}}\n\n" +
		"data: [DONE]\n\n"
	p, cTok, total := tokenCountsFromSSEContent(sse)
	if p != 12 || cTok != 3 || total != 15 {
		t.Fatalf("counts = %d/%d/%d, want 12/3/15", p, cTok, total)
	}
}

func TestApproximateTokensFromPassthroughContent_UsesDeltaText(t *testing.T) {
	// 8 runes → 2 tokens at /4
	sse := "data: {\"choices\":[{\"delta\":{\"content\":\"abcdefgh\"}}]}\n\n" +
		"data: [DONE]\n\n"
	n := approximateTokensFromPassthroughContent(sse)
	if n != 2 {
		t.Fatalf("approx = %d, want 2", n)
	}
}

func TestSanitizeUsageModel(t *testing.T) {
	cases := map[string]string{
		"pipeline.transparent-proxy": "",
		"pipeline.direct-backend":    "",
		"pipeline_router-mode":       "",
		"centag/transparent-proxy":   "",
		"deepseek-v4-flash":          "deepseek-v4-flash",
		"glm-4-flash":                "glm-4-flash",
		"{{system.default_model}}":   "",
		"":                          "",
	}
	for in, want := range cases {
		if got := sanitizeUsageModel(in); got != want {
			t.Errorf("sanitizeUsageModel(%q) = %q, want %q", in, got, want)
		}
	}
}

