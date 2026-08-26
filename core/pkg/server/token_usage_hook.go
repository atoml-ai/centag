package server

import (
	"context"

	"centag/core/internal/tokenusage"
	"centag/core/pkg/hooks"
	"centag/core/pkg/logger"
	"centag/core/pkg/pipeline"
)

func wireTokenUsagePersistence(svc *tokenusage.Service, hm hooks.HookManager) {
	if svc == nil {
		return
	}
	existingMetrics := pipeline.RecordSchedulerMetrics
	pipeline.PersistTokenUsage = func(ctx context.Context, req pipeline.TokenUsagePersistRequest) {
		usage := &hooks.TokenUsage{
			UserID:       req.UserID,
			APIKeyID:     req.APIKeyID,
			TenantID:     req.TenantID,
			RequestID:    req.RequestID,
			Model:        req.Model,
			Backend:      req.BackendID,
			InputTokens:  req.PromptTokens,
			OutputTokens: req.CompletionTokens,
			TotalTokens:  req.TotalTokens,
			Success:      true,
			DeptTag:      req.DeptTag,
			AgentType:    req.AgentType,
			SessionID:    req.SessionID, // 039: 会话 ID
			Source:       req.Source, // "cache_replay" = 缓存命中回放计量
		}
		if hm != nil {
			_ = hm.TriggerTokenUsedHooks(ctx, usage)
		} else {
			adapter := newTokenUsageHookAdapter(svc)
			if err := adapter.OnTokenUsed(ctx, usage); err != nil {
				logger.Warnf("[TokenUsage] persist failed: user_id=%d model=%s tokens=%d err=%v",
					req.UserID, req.Model, req.TotalTokens, err)
				return
			}
		}
		if existingMetrics != nil && req.BackendID != "" {
			existingMetrics(req.BackendID, req.Model, 0, true)
		}
	}
}
