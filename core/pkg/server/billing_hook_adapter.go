package server

import (
	"context"

	"centag/core/internal/billing"
	"centag/core/pkg/hooks"
	"centag/core/pkg/logger"
)

// billingHookAdapter forwards token usage to the async billing.Service (team).
type billingHookAdapter struct {
	svc *billing.Service
}

func newBillingHookAdapter(svc *billing.Service) *billingHookAdapter {
	return &billingHookAdapter{svc: svc}
}

func (a *billingHookAdapter) OnUsage(ctx context.Context, usage *hooks.TokenUsage) error {
	if a == nil || a.svc == nil || usage == nil {
		return nil
	}
	ev := billing.NewRequestEvent(usage.UserID, usage.TenantID, usage.Backend, usage.Model, int64(usage.TotalTokens))
	if usage.CostUSD > 0 {
		ev.Amount = usage.CostUSD
		ev.Type = "token_usage"
	}
	a.svc.RecordEvent(ev)
	return nil
}

func (a *billingHookAdapter) OnQuotaExceeded(ctx context.Context, userID int64) error {
	logger.Warnf("[billing] quota exceeded for user_id=%d", userID)
	return nil
}
