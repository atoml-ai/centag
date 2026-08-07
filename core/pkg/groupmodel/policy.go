// Package groupmodel resolves the effective Team policy for a user under the
// group model (migration 036 / 040).
//
// Model:
//   - plan_templates hold reusable metering rules (quotas, price_type, rate limits);
//   - groups hold resource allowlists (available backends / models / pipelines);
//   - users/groups are assigned to a template (user_plan_assignments /
//     group_plan_assignments);
//   - policy_mode=group inherits the group's template + group resource scope;
//     metering_mode selects per_member or shared_pool;
//   - policy_mode=custom uses the user's template for metering only (no resource
//     allowlist — Team should force users into a group);
//   - Team normal users without an assignment are denied (fail-closed);
//   - Personal / Minimal use SyntheticFullAccess() (no DB template required).
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

// Group metering modes (group_plan_assignments.metering_mode).
const (
	MeteringPerMember  = "per_member"
	MeteringSharedPool = "shared_pool"
)

// EffectivePolicy is the normalized, resolved policy for a user at a point in
// time. Every enforcement gate reads the same struct, so the "custom vs group
// inheritance" decision happens exactly once.
type EffectivePolicy struct {
	Mode    string // PolicyModeGroup, PolicyModeCustom, or "" (no policy)
	GroupID string // resolved group in group mode ("" in single-user mode)

	// Resource allowlists. When ResourcesConfigured is true, empty lists mean
	// all allowed. When false, IsAllowed* denies (avoids custom/no-scope full-open).
	AllowBackends        []string
	AllowModels          []string
	AllowPipelines       []string
	ResourcesConfigured  bool

	PriceType string // billing price type ("cost" | "revenue")

	// Budget (money quota; column chosen by PriceType: revenue_usd or cost_usd).
	BudgetAmount  *float64
	BudgetPeriod  string // monthly | yearly | custom
	BudgetStartAt *time.Time
	BudgetEndAt   *time.Time

	// Token quota (prompt / completion / total within the window).
	TokenQuotaInput   *int64
	TokenQuotaOutput  *int64
	TokenQuotaTotal   *int64
	TokenQuotaPeriod  string // daily | monthly | yearly | custom
	TokenQuotaStartAt *time.Time
	TokenQuotaEndAt   *time.Time

	// Rate limits (per minute).
	RateLimitRPM int
	RateLimitTPM int

	// Group shared-pool limits. Populated in group mode from group_quotas.
	// 0 = unlimited.
	GroupDailyTokenLimit     int64
	GroupDailyRequestLimit   int64
	GroupMonthlyTokenLimit   int64
	GroupMonthlyRequestLimit int64
	GroupMaxBackends         int
	GroupMaxAPIKeys          int

	// Template identity (empty when synthetic or legacy fallback without template).
	TemplateID   int64
	TemplateName string

	// MeteringMode: per_member (default) or shared_pool. Only meaningful in group mode.
	MeteringMode string

	// HasPlan reports whether an active template assignment (or legacy plan)
	// was resolved, or a synthetic full-access policy was injected.
	HasPlan bool
}

// SyntheticFullAccess returns the Personal/Minimal edition policy: all
// resources allowed, no budget / token / rate limits.
func SyntheticFullAccess() *EffectivePolicy {
	return &EffectivePolicy{
		Mode:                PolicyModeCustom,
		HasPlan:             true,
		ResourcesConfigured: true, // empty allowlists + configured = all allowed
		PriceType:           "cost",
	}
}

// IsGroup reports whether the policy inherits a group's assignment.
func (p *EffectivePolicy) IsGroup() bool {
	return p != nil && p.Mode == PolicyModeGroup && p.GroupID != ""
}

// UsesSharedPool reports whether budget/token sums should cover the whole group.
func (p *EffectivePolicy) UsesSharedPool() bool {
	return p != nil && p.IsGroup() && p.MeteringMode == MeteringSharedPool
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
		(p.TokenQuotaOutput != nil && *p.TokenQuotaOutput > 0) ||
		(p.TokenQuotaTotal != nil && *p.TokenQuotaTotal > 0)
}

// IsAllowedBackend reports whether a backend is in the allowlist.
func (p *EffectivePolicy) IsAllowedBackend(backendID string) bool {
	if p == nil || !p.ResourcesConfigured {
		return false
	}
	if len(p.AllowBackends) == 0 {
		return true // empty + configured = all allowed
	}
	return contains(p.AllowBackends, backendID)
}

// IsAllowedModel reports whether a model is in the allowlist.
func (p *EffectivePolicy) IsAllowedModel(model string) bool {
	if p == nil || !p.ResourcesConfigured {
		return false
	}
	if len(p.AllowModels) == 0 {
		return true
	}
	if contains(p.AllowModels, model) {
		return true
	}
	return contains(p.AllowModels, "*")
}

// IsAllowedPipeline reports whether a pipeline is in the allowlist.
func (p *EffectivePolicy) IsAllowedPipeline(pipelineID string) bool {
	if p == nil || !p.ResourcesConfigured {
		return false
	}
	if len(p.AllowPipelines) == 0 {
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
