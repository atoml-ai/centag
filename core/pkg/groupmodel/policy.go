// Package groupmodel resolves the effective Team policy for a user under the
// group model (migration 036).
//
// Model:
//   - users are independent metering / pricing / permission units;
//   - groups are N:1 policy containers (shared metering pool + pricing rules
//     + resource allowlist);
//   - a user inherits the group's rules when policy_mode = "group", or uses
//     their own user_plans / user_pricing_overrides when policy_mode = "custom";
//   - with no active plan the global default (unlimited) applies.
//
// The package is shared: open-core middleware and centag-pro enforcement
// points both consume the same resolver, so a single policy lookup keeps every
// gate consistent. It only speaks SQL against the shared database, so it can
// live in open-core without importing centag-pro.
package groupmodel

import (
	"strings"
	"time"
)

// Policy mode values (users.policy_mode).
const (
	PolicyModeGroup  = "group"
	PolicyModeCustom = "custom"
)

// EffectivePolicy is the normalized, resolved policy for a user at a point in
// time. Every enforcement gate reads the same struct, so the "custom vs group
// inheritance" decision happens exactly once.
type EffectivePolicy struct {
	Mode    string // PolicyModeGroup, PolicyModeCustom, or "" (no policy)
	GroupID string // resolved group in group mode ("" in single-user mode)

	// Resource allowlists (empty = all allowed).
	AllowBackends  []string
	AllowModels    []string
	AllowPipelines []string

	PriceType string // billing price type ("cost" | "revenue")

	// Budget (money quota, enforced on token_usage.cost_usd within the window).
	BudgetAmount   *float64
	BudgetPeriod   string // monthly | yearly | custom
	BudgetStartAt  *time.Time
	BudgetEndAt    *time.Time

	// Token quota (prompt / completion within the window).
	TokenQuotaInput   *int64
	TokenQuotaOutput  *int64
	TokenQuotaPeriod  string // monthly | yearly | custom
	TokenQuotaStartAt *time.Time
	TokenQuotaEndAt   *time.Time

	// Rate limits (per minute).
	RateLimitRPM int
	RateLimitTPM int

	// User-level token limits. Populated in custom mode from the users table.
	// 0 = unlimited.
	DailyTokenLimit   int64
	MonthlyTokenLimit int64

	// Group shared-pool limits. Populated in group mode from group_quotas.
	// 0 = unlimited.
	GroupDailyTokenLimit    int64
	GroupDailyRequestLimit  int64
	GroupMonthlyTokenLimit  int64
	GroupMonthlyRequestLimit int64
	GroupMaxBackends        int
	GroupMaxAPIKeys         int

	// HasPlan reports whether an active plan row (group or user) was resolved.
	HasPlan bool
}

// IsGroup reports whether the policy inherits a group's shared pool.
func (p *EffectivePolicy) IsGroup() bool {
	return p != nil && p.Mode == PolicyModeGroup && p.GroupID != ""
}

// IsBudgetEnabled mirrors the plan helper (used by enforcement points).
func (p *EffectivePolicy) IsBudgetEnabled() bool {
	return p != nil && p.BudgetAmount != nil && *p.BudgetAmount > 0
}

// IsTokenQuotaEnabled mirrors the plan helper.
func (p *EffectivePolicy) IsTokenQuotaEnabled() bool {
	if p == nil {
		return false
	}
	return (p.TokenQuotaInput != nil && *p.TokenQuotaInput > 0) ||
		(p.TokenQuotaOutput != nil && *p.TokenQuotaOutput > 0)
}

// IsAllowedBackend reports whether a backend is in the allowlist.
func (p *EffectivePolicy) IsAllowedBackend(backendID string) bool {
	if p == nil || len(p.AllowBackends) == 0 {
		return true // empty = all allowed
	}
	return contains(p.AllowBackends, backendID)
}

// IsAllowedModel reports whether a model is in the allowlist.
func (p *EffectivePolicy) IsAllowedModel(model string) bool {
	if p == nil || len(p.AllowModels) == 0 {
		return true
	}
	if contains(p.AllowModels, model) {
		return true
	}
	return contains(p.AllowModels, "*")
}

// IsAllowedPipeline reports whether a pipeline is in the allowlist.
func (p *EffectivePolicy) IsAllowedPipeline(pipelineID string) bool {
	if p == nil || len(p.AllowPipelines) == 0 {
		return true
	}
	return contains(p.AllowPipelines, pipelineID)
}

// PricingOverride is a per-(backend, model, price_type) override from the
// user's or the user's group's override table.
type PricingOverride struct {
	BackendID       string
	Model           string
	PriceType       string
	InputPricePerM  float64
	OutputPricePerM float64
	Currency        string
}

func contains(items []string, s string) bool {
	for _, item := range items {
		if strings.TrimSpace(item) == strings.TrimSpace(s) {
			return true
		}
	}
	return false
}
