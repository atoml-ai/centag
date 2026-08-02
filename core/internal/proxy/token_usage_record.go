package proxy

import (
	"context"
	"encoding/json"
	"strings"

	"centag/core/internal/auth"
	"centag/core/internal/tokenusage"
	"centag/core/pkg/hooks"
	"centag/core/pkg/pipeline"
	"github.com/gin-gonic/gin"
)

// maybeRecordTokenUsage persists usage for proxy requests when the pipeline has no
// token_usage node (e.g. direct-backend). Pipelines that already run TokenUsageNode
// are left to the pipeline hook to avoid double counting.
// Persistence goes through HookManager.TriggerTokenUsedHooks when available.
func maybeRecordTokenUsage(c *gin.Context, output *pipeline.PipelineOutput, fallbackModel string) {
	if c == nil || output == nil || pipelineDelegatedTokenUsage(output) {
		return
	}

	userID, err := auth.GetUserID(c)
	if err != nil {
		userID = 0
	}

	prompt, completion, total := tokenCountsFromOutput(output)
	if total <= 0 {
		// transparent_forward 流式：Content 常为完整上游 SSE，优先解析 usage 字段
		if p, cTok, t := tokenCountsFromSSEContent(output.Content); t > 0 {
			prompt, completion, total = p, cTok, t
		}
	}
	if total <= 0 {
		content := strings.TrimSpace(output.Content)
		if content == "" {
			return
		}
		// 去掉 SSE 前缀后估算，避免 data:/JSON 噪声夸大
		approx := approximateTokensFromPassthroughContent(content)
		if approx <= 0 {
			return
		}
		prompt = approx
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

	clientIP := c.ClientIP()
	usage := &hooks.TokenUsage{
		UserID:       userID,
		APIKeyID:     auth.GetAPIKeyID(c),
		TenantID:     auth.GetTenantID(c),
		RequestID:    c.GetHeader("X-Request-ID"),
		SessionID:    strings.TrimSpace(c.GetHeader("X-Session-ID")),
		Model:        model,
		Backend:      extractBackendFromPipelineOutput(output),
		InputTokens:  prompt,
		OutputTokens: completion,
		TotalTokens:  total,
		Success:      output.ExecutionLog == nil || output.ExecutionLog.Success,
		DeptTag:      deptTag,
	}

	go func() {
		ctx := context.Background()
		if hm := hooks.Default(); hm != nil {
			_ = hm.TriggerTokenUsedHooks(ctx, usage)
		} else if svc := tokenusage.DefaultService(); svc != nil {
			// Fallback when HookManager not wired (tests / minimal harness).
			_ = svc.RecordUsage(ctx, &tokenusage.UsageRecord{
				UserID:           usage.UserID,
				BackendID:        usage.Backend,
				Model:            usage.Model,
				PromptTokens:     usage.InputTokens,
				CompletionTokens: usage.OutputTokens,
				TotalTokens:      usage.TotalTokens,
				TenantID:         usage.TenantID,
				DeptTag:          usage.DeptTag,
				RequestID:        usage.RequestID,
				ClientIP:         clientIP,
				Success:          usage.Success,
			})
		}
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

// tokenCountsFromSSEContent extracts OpenAI-style usage from buffered SSE / JSON body.
func tokenCountsFromSSEContent(content string) (prompt, completion, total int) {
	content = strings.TrimSpace(content)
	if content == "" {
		return 0, 0, 0
	}
	type usageObj struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
		InputTokens      int `json:"input_tokens"`  // some providers
		OutputTokens     int `json:"output_tokens"` // some providers
	}
	type envelope struct {
		Usage *usageObj `json:"usage"`
	}

	tryParse := func(raw string) (int, int, int, bool) {
		raw = strings.TrimSpace(raw)
		if raw == "" || raw == "[DONE]" {
			return 0, 0, 0, false
		}
		var env envelope
		if err := json.Unmarshal([]byte(raw), &env); err != nil || env.Usage == nil {
			return 0, 0, 0, false
		}
		u := env.Usage
		p, cTok, t := u.PromptTokens, u.CompletionTokens, u.TotalTokens
		if p == 0 && u.InputTokens > 0 {
			p = u.InputTokens
		}
		if cTok == 0 && u.OutputTokens > 0 {
			cTok = u.OutputTokens
		}
		if t == 0 {
			t = p + cTok
		}
		if t <= 0 {
			return 0, 0, 0, false
		}
		return p, cTok, t, true
	}

	// Full JSON response
	if p, cTok, t, ok := tryParse(content); ok {
		return p, cTok, t
	}
	// SSE: scan data: lines reverse-ish (usage usually near end)
	lines := strings.Split(content, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if p, cTok, t, ok := tryParse(payload); ok {
			return p, cTok, t
		}
	}
	return 0, 0, 0
}

func approximateTokensFromPassthroughContent(content string) int {
	var b strings.Builder
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "data:") {
			payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if payload == "" || payload == "[DONE]" {
				continue
			}
			var chunk struct {
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
					Message struct {
						Content string `json:"content"`
					} `json:"message"`
				} `json:"choices"`
			}
			if err := json.Unmarshal([]byte(payload), &chunk); err == nil {
				for _, ch := range chunk.Choices {
					if ch.Delta.Content != "" {
						b.WriteString(ch.Delta.Content)
					} else if ch.Message.Content != "" {
						b.WriteString(ch.Message.Content)
					}
				}
				continue
			}
		}
	}
	text := b.String()
	if text == "" {
		// non-SSE fallback
		text = content
	}
	n := len([]rune(text)) / 4
	if n < 1 && len(text) > 0 {
		return 1
	}
	return n
}
