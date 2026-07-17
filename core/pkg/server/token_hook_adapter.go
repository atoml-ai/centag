package server

import (
	"context"

	"centag/core/internal/tokenusage"
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
	rec := &tokenusage.UsageRecord{
		UserID:           usage.UserID,
		BackendID:        usage.Backend,
		Model:            usage.Model,
		PromptTokens:     usage.InputTokens,
		CompletionTokens: usage.OutputTokens,
		TotalTokens:      usage.TotalTokens,
		CostUSD:          usage.CostUSD,
		Success:          usage.Success,
		TenantID:         usage.TenantID,
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
