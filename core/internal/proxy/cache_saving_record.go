package proxy

import (
	"context"
	"strings"

	"github.com/gin-gonic/gin"
	"centag/core/internal/auth"
	"centag/core/pkg/pipeline"
	"centag/core/internal/tokenusage"
)

// maybeRecordCacheSaving records estimated upstream cost avoided on pipeline cache hits.
func maybeRecordCacheSaving(c *gin.Context, output *pipeline.PipelineOutput, fallbackModel string) {
	if c == nil || output == nil || !pipelineCacheHit(output) {
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

	model := extractModelFromPipelineOutput(output)
	if model == "" {
		model = strings.TrimSpace(fallbackModel)
	}
	model = sanitizeUsageModel(model)
	backendID := extractBackendFromPipelineOutput(output)

	prompt, completion, total := tokenusage.EstimateSavedTokensFromResponse(output.Content, 0)

	deptTag := strings.TrimSpace(c.GetHeader("X-Dept-Tag"))
	if deptTag == "" && output.Metadata != nil {
		if v, ok := output.Metadata["dept_tag"].(string); ok {
			deptTag = strings.TrimSpace(v)
		}
	}

	pipelineID := ""
	if output.ExecutionLog != nil {
		pipelineID = strings.TrimSpace(output.ExecutionLog.PipelineID)
	}

	record := &tokenusage.CacheSavingRecord{
		UserID:           userID,
		BackendID:        backendID,
		Model:            model,
		PromptTokens:     prompt,
		CompletionTokens: completion,
		TotalTokens:      total,
		CacheLayer:       cacheLayerFromOutput(output),
		TenantID:         auth.GetTenantID(c),
		DeptTag:          deptTag,
		RequestID:        c.GetHeader("X-Request-ID"),
		PipelineID:       pipelineID,
	}
	go func() {
		_ = svc.RecordCacheSaving(context.Background(), record)
	}()
}

func pipelineCacheHit(output *pipeline.PipelineOutput) bool {
	if output == nil {
		return false
	}
	if output.Metadata != nil {
		if hit, ok := output.Metadata["cache_hit"].(bool); ok && hit {
			return true
		}
	}
	return false
}

func cacheLayerFromOutput(output *pipeline.PipelineOutput) string {
	if output == nil || output.Metadata == nil {
		return "L1"
	}
	if strategy, _ := output.Metadata["strategy"].(string); strings.EqualFold(strategy, "semantic") {
		return "L2"
	}
	if score, ok := output.Metadata["cache_score"].(float64); ok && score > 0 {
		return "L2"
	}
	return "L1"
}