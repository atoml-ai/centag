package server

import (
	"context"
	"log"

	"centag/core/pkg/pipeline"
	"centag/core/internal/tokenusage"
)

func wireTokenUsagePersistence(svc *tokenusage.Service) {
	if svc == nil {
		return
	}
	existingMetrics := pipeline.RecordSchedulerMetrics
	pipeline.PersistTokenUsage = func(ctx context.Context, req pipeline.TokenUsagePersistRequest) {
		rec := &tokenusage.UsageRecord{
			UserID:           req.UserID,
			BackendID:        req.BackendID,
			Model:            req.Model,
			PromptTokens:     req.PromptTokens,
			CompletionTokens: req.CompletionTokens,
			TotalTokens:      req.TotalTokens,
			TenantID:         req.TenantID,
			DeptTag:          req.DeptTag,
			RequestID:        req.RequestID,
			AgentType:        req.AgentType,
			Success:          true,
		}
		if err := svc.RecordUsage(ctx, rec); err != nil {
			log.Printf("[TokenUsage] persist failed: user_id=%d model=%s tokens=%d err=%v",
				req.UserID, req.Model, req.TotalTokens, err)
			return
		}
		if existingMetrics != nil && req.BackendID != "" {
			existingMetrics(req.BackendID, req.Model, 0, true)
		}
	}
}