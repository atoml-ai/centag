package proxy

import (
	"context"
	"strings"

	"github.com/gin-gonic/gin"

	"centag/core/internal/auth"
	"centag/core/pkg/hooks"
	"centag/core/pkg/pipeline"
	"centag/core/pkg/plugin"
	"centag/core/pkg/types"
)

// resolveSessionID prefers X-Session-ID, else creates a new ephemeral id from request id.
func resolveSessionID(c *gin.Context) string {
	if c == nil {
		return ""
	}
	if sid := strings.TrimSpace(c.GetHeader("X-Session-ID")); sid != "" {
		return sid
	}
	if rid := strings.TrimSpace(c.GetHeader("X-Request-ID")); rid != "" {
		return "req_" + rid
	}
	return ""
}

// triggerConversationRequestHooks notifies conversation StorageHooks before pipeline run.
func triggerConversationRequestHooks(c *gin.Context, req *plugin.ProxyRequest, mode ProxyMode, pipelineID string) string {
	hm := hooks.Default()
	if hm == nil || c == nil || req == nil {
		return resolveSessionID(c)
	}
	sessionID := resolveSessionID(c)
	userID, _ := auth.GetUserID(c)
	category := strings.TrimSpace(c.GetHeader("X-Conversation-Category"))
	if category == "" {
		category = "general"
	}
	userContent := extractQuestionFromMessages(req.Messages)
	ureq := &types.UnifiedRequest{
		Model:  req.Model,
		Stream: req.Stream,
		Metadata: map[string]interface{}{
			"session_id":   sessionID,
			"user_id":      userID,
			"tenant_id":    auth.GetTenantID(c),
			"category":     category,
			"pipeline_id":  pipelineID,
			"proxy_mode":   string(mode),
			"request_id":   c.GetHeader("X-Request-ID"),
			"user_content": userContent,
		},
	}
	_ = hm.TriggerRequestHooks(c.Request.Context(), ureq)
	if sid, ok := ureq.Metadata["session_id"].(string); ok && sid != "" {
		sessionID = sid
	}
	if sessionID != "" {
		c.Header("X-Session-ID", sessionID)
		c.Set("conversation_session_id", sessionID)
	}
	return sessionID
}

// triggerConversationResponseHooks records the completed turn (stream or non-stream).
func triggerConversationResponseHooks(c *gin.Context, output *pipeline.PipelineOutput, model, sessionID string) {
	hm := hooks.Default()
	if hm == nil || c == nil {
		return
	}
	if sessionID == "" {
		if v, ok := c.Get("conversation_session_id"); ok {
			sessionID, _ = v.(string)
		}
	}
	if sessionID == "" {
		sessionID = resolveSessionID(c)
	}
	content := ""
	backend := ""
	inTok, outTok := 0, 0
	if output != nil {
		content = output.Content
		backend = extractBackendFromPipelineOutput(output)
		inTok, outTok, _ = tokenCountsFromOutput(output)
		if m := extractModelFromPipelineOutput(output); m != "" {
			model = m
		}
		// prefer router category when present
	}
	category := strings.TrimSpace(c.GetHeader("X-Conversation-Category"))
	if category == "" {
		category = "general"
	}
	if output != nil && output.Metadata != nil {
		if cat, ok := output.Metadata["category"].(string); ok && strings.TrimSpace(cat) != "" {
			category = strings.TrimSpace(cat)
		}
		if cat, ok := output.Metadata["route_category"].(string); ok && strings.TrimSpace(cat) != "" {
			category = strings.TrimSpace(cat)
		}
	}

	resp := &types.UnifiedResponse{
		Content:    content,
		Model:      model,
		TokensUsed: outTok,
		Metadata: map[string]interface{}{
			"session_id":    sessionID,
			"request_id":    c.GetHeader("X-Request-ID"),
			"pipeline_id":   c.GetHeader("X-Pipeline-ID"),
			"backend":       backend,
			"category":      category,
			"status_code":   200,
			"input_tokens":  inTok,
			"output_tokens": outTok,
			"user_id":       mustUserID(c),
			"tenant_id":     auth.GetTenantID(c),
		},
	}
	_ = hm.TriggerResponseHooks(context.Background(), resp)
	if sessionID != "" {
		c.Header("X-Session-ID", sessionID)
	}
}

func mustUserID(c *gin.Context) int64 {
	id, err := auth.GetUserID(c)
	if err != nil {
		return 0
	}
	return id
}
