package proxy

import (
	"context"
	"strings"

	"github.com/gin-gonic/gin"
	"centag/core/internal/auth"
	"centag/core/pkg/pipeline"
	"centag/core/internal/tokenusage"
)

// maybeRecordTokenUsage persists usage for proxy requests when the pipeline has no
// token_usage node (e.g. direct-backend). Pipelines that already run TokenUsageNode
// are left to the pipeline hook to avoid double counting.
func maybeRecordTokenUsage(c *gin.Context, output *pipeline.PipelineOutput, fallbackModel string) {
	if c == nil || output == nil || pipelineDelegatedTokenUsage(output) {
		return
	}
	svc := tokenusage.DefaultService()
	if svc == nil {
		return
	}
	userID, err := auth.GetUserID(c)
	if err != nil || userID == 0 {
		return
	}

	prompt, completion, total := tokenCountsFromOutput(output)
	if total <= 0 {
		content := strings.TrimSpace(output.Content)
		if content == "" {
			return
		}
		prompt = len(content) / 4
		total = prompt
	}

	model := extractModelFromPipelineOutput(output)
	if model == "" {
		model = strings.TrimSpace(fallbackModel)
	}

	deptTag := strings.TrimSpace(c.GetHeader("X-Dept-Tag"))
	if deptTag == "" && output.Metadata != nil {
		if v, ok := output.Metadata["dept_tag"].(string); ok {
			deptTag = strings.TrimSpace(v)
		}
	}

	record := &tokenusage.UsageRecord{
		UserID:           userID,
		BackendID:        extractBackendFromPipelineOutput(output),
		Model:            model,
		PromptTokens:     prompt,
		CompletionTokens: completion,
		TotalTokens:      total,
		TenantID:         auth.GetTenantID(c),
		DeptTag:          deptTag,
		RequestID:        c.GetHeader("X-Request-ID"),
		ClientIP:         c.ClientIP(),
		Success:          output.ExecutionLog == nil || output.ExecutionLog.Success,
	}
	go func() {
		_ = svc.RecordUsage(context.Background(), record)
		pipeline.RecordSchedulerMetricsFromOutput(output)
	}()
}

// pipelineDelegatedTokenUsage reports whether a successful token_usage node already
// recorded usage from a successful upstream LLM node (skip proxy fallback to avoid double count).
func pipelineDelegatedTokenUsage(output *pipeline.PipelineOutput) bool {
	if output == nil || output.ExecutionLog == nil {
		return false
	}
	hasTokenUsage := false
	hasLLMTokens := false
	for _, nl := range output.ExecutionLog.NodeLogs {
		if !nl.Success {
			continue
		}
		switch nl.NodeType {
		case pipeline.NodeTypeTokenUsage:
			hasTokenUsage = true
		case pipeline.NodeTypeGenerator, pipeline.NodeTypeTransparentForward:
			if nl.InputTokens > 0 || nl.OutputTokens > 0 {
				hasLLMTokens = true
			}
		}
	}
	return hasTokenUsage && hasLLMTokens
}

func tokenCountsFromOutput(output *pipeline.PipelineOutput) (prompt, completion, total int) {
	if output == nil || output.ExecutionLog == nil {
		return 0, 0, 0
	}
	log := output.ExecutionLog
	for _, nl := range log.NodeLogs {
		if !nl.Success {
			continue
		}
		switch nl.NodeType {
		case pipeline.NodeTypeGenerator, pipeline.NodeTypeTransparentForward:
			if nl.InputTokens > 0 || nl.OutputTokens > 0 {
				prompt += nl.InputTokens
				completion += nl.OutputTokens
			}
		}
	}
	if prompt+completion > 0 {
		return prompt, completion, prompt + completion
	}
	if log.TotalTokens > 0 {
		half := log.TotalTokens / 2
		return half, log.TotalTokens - half, log.TotalTokens
	}
	return 0, 0, 0
}