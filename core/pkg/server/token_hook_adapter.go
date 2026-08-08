package server

import (
	"context"

	"centag/core/internal/tokenusage"
	"centag/core/pkg/backend"
	"centag/core/pkg/hooks"
)

// tokenUsageHookAdapter persists TokenUsage via tokenusage.Service (TokenHook).
type tokenUsageHookAdapter struct {
	svc *tokenusage.Service
}

func newTokenUsageHookAdapter(svc *tokenusage.Service) *tokenUsageHookAdapter {
	return &tokenUsageHookAdapter{svc: svc}
}

func (a *tokenUsageHookAdapter) OnTokenUsed(ctx context.Context, usage *hooks.TokenUsage) error {
	if a == nil || a.svc == nil || usage == nil {
		return nil
	}
	// 用户自建后端（TenantID 非空 = 用户私有）：用量归属到用户 scope，
	// 使团队 tenant 配额与组共享池配额均不计入（用量仍正常记录展示）。
	// 覆盖 pipeline token_usage 节点（PersistTokenUsage）等所有 hook 入口。
	if scope := backend.UserOwnedScope(usage.Backend); scope != "" {
		usage.TenantID = scope
		usage.GroupID = scope
	}
	rec := &tokenusage.UsageRecord{
		UserID:           usage.UserID,
		APIKeyID:         usage.APIKeyID,
		BackendID:        usage.Backend,
		Model:            usage.Model,
		PromptTokens:     usage.InputTokens,
		CompletionTokens: usage.OutputTokens,
		TotalTokens:      usage.TotalTokens,
		CostUSD:          usage.CostUSD,
		Success:          usage.Success,
		TenantID:         usage.TenantID,
		GroupID:          usage.GroupID,
		DeptTag:          usage.DeptTag,
		RequestID:        usage.RequestID,
		AgentType:        usage.AgentType,
	}
	if rec.TotalTokens == 0 && !rec.Success {
		// allow explicit failure rows with zero tokens
	} else if rec.TotalTokens <= 0 {
		return nil
	}
	return a.svc.RecordUsage(ctx, rec)
}
