package pipeline

import (
	"context"
	"log"
	"strconv"
	"strings"
)

// TokenUsagePersistRequest is passed to the optional usage persistence hook.
type TokenUsagePersistRequest struct {
	UserID           int64
	APIKeyID         int64 // 命中的虚拟 Key 主键；0 = JWT 认证或未传播（落库时转 NULL）
	BackendID        string
	Model            string
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	TenantID         string
	DeptTag          string
	RequestID        string
	AgentType        string
	SessionID        string // 039: 会话 ID
	Source           string // 计量来源："cache_replay"=缓存命中恢复的计量（非真实后端消耗）；"" = 默认/真实调用
}

// PersistTokenUsage optionally records token usage to persistent storage.
// Wired from server startup to avoid an import cycle with tokenusage.
var PersistTokenUsage func(ctx context.Context, req TokenUsagePersistRequest)

func persistTokenUsageFromRecord(ctx context.Context, input *NodeInput, record map[string]interface{}) {
	if PersistTokenUsage == nil {
		return
	}
	total := tokenRecordInt(record["total_tokens"])
	if total <= 0 {
		return
	}
	req := TokenUsagePersistRequest{
		BackendID:        sanitizeUsageValue(tokenRecordString(record["backend_id"])),
		Model:            sanitizeUsageValue(tokenRecordString(record["model"])),
		PromptTokens:     tokenRecordInt(record["prompt_tokens"]),
		CompletionTokens: tokenRecordInt(record["completion_tokens"]),
		TotalTokens:      total,
		RequestID:        tokenRecordString(record["request_id"]),
	}
	if input != nil && input.Metadata != nil {
		req.TenantID = tokenRecordString(input.Metadata["tenant_id"])
		req.DeptTag = tokenRecordString(input.Metadata["dept_tag"])
		req.AgentType = tokenRecordString(input.Metadata["agent_type"])
		req.SessionID = tokenRecordString(input.Metadata["session_id"]) // 039: 会话 ID
		req.APIKeyID = int64(tokenRecordInt(input.Metadata["api_key_id"]))
		if replay, _ := input.Metadata["cache_replay"].(bool); replay {
			req.Source = "cache_replay"
		}
		if uid := tokenRecordString(input.Metadata["user_id"]); uid != "" {
			if id, err := parseUserIDInt(uid); err == nil {
				req.UserID = id
			}
		}
		if req.RequestID == "" {
			req.RequestID = tokenRecordString(input.Metadata["request_id"])
		}
	}
	if req.UserID == 0 {
		if execCtx, ok := ctx.Value(executionContextKey{}).(*ExecutionContext); ok && execCtx != nil {
			if uid, ok := execCtx.GetVariable("user_id"); ok {
				if id, err := parseUserIDInt(tokenRecordString(uid)); err == nil {
					req.UserID = id
				} else if n := tokenRecordInt(uid); n > 0 {
					req.UserID = int64(n)
				}
			}
		}
	}
	if req.UserID == 0 {
		log.Printf("[TokenUsage] skip persist: missing user_id (model=%s tokens=%d)", req.Model, total)
		return
	}
	go func() {
		PersistTokenUsage(context.Background(), req)
	}()
}

func parseUserIDInt(uid string) (int64, error) {
	return strconv.ParseInt(uid, 10, 64)
}

// sanitizeUsageValue returns empty string for unresolved template variables or
// non-concrete identifiers, preventing them from being stored in usage tables.
func sanitizeUsageValue(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || strings.Contains(s, "{{") {
		return ""
	}
	return s
}
