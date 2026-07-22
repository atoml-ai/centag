package config

import (
	"time"
)

// GlobalFallbackPolicy 全局降级策略（替代原 pipeline 内联 fallback_groups）。
type GlobalFallbackPolicy struct {
	ID          string                `json:"id" db:"id"`
	Name        string                `json:"name" db:"name"`
	Description string                `json:"description,omitempty" db:"description"`
	Strategy    FallbackStrategyType  `json:"strategy" db:"strategy"`
	Rules       []FallbackRule        `json:"rules"`
	Enabled     bool                  `json:"enabled" db:"enabled"`
	CreatedAt   time.Time             `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time             `json:"updated_at" db:"updated_at"`
}

// FallbackStrategyType 降级策略类型。
type FallbackStrategyType string

const (
	// StrategySameModelDifferentBackend 同模型不同后端：模型名不变，按优先级切换后端。
	StrategySameModelDifferentBackend FallbackStrategyType = "same_model_different_backend"
	// StrategySameBackendDifferentModel 同后端不同模型：后端不变，按优先级切换模型。
	StrategySameBackendDifferentModel FallbackStrategyType = "same_backend_different_model"
	// StrategyCustomChain 自定义降级链：手动指定 backend+model 对。
	StrategyCustomChain FallbackStrategyType = "custom_chain"
)

// FallbackRule 降级规则中的一级。
type FallbackRule struct {
	Priority   int    `json:"priority"`
	BackendID  string `json:"backend_id"`
	Model      string `json:"model"`                  // "{{requested_model}}" 表示透传客户端请求的模型
	TimeoutSec int    `json:"timeout_sec,omitempty"`  // 该级降级的超时（0=使用全局默认）
	MaxRetries int    `json:"max_retries,omitempty"`  // 该级最大重试次数（0=使用全局默认）
}

// Validate 校验策略合法性。
func (p *GlobalFallbackPolicy) Validate() error {
	if p.ID == "" {
		return ErrFallbackPolicyNoID
	}
	if p.Name == "" {
		return ErrFallbackPolicyNoName
	}
	switch p.Strategy {
	case StrategySameModelDifferentBackend, StrategySameBackendDifferentModel, StrategyCustomChain:
	default:
		return ErrFallbackPolicyInvalidStrategy
	}
	if len(p.Rules) == 0 {
		return ErrFallbackPolicyNoRules
	}
	for i, r := range p.Rules {
		if r.BackendID == "" {
			return ErrFallbackRuleNoBackend
		}
		if r.Model == "" {
			return ErrFallbackRuleNoModel
		}
		if r.Priority <= 0 {
			p.Rules[i].Priority = i + 1
		}
	}
	return nil
}

// SortedRules 返回按 Priority 升序排列的规则副本。
func (p *GlobalFallbackPolicy) SortedRules() []FallbackRule {
	if len(p.Rules) <= 1 {
		return p.Rules
	}
	out := make([]FallbackRule, len(p.Rules))
	copy(out, p.Rules)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].Priority < out[j-1].Priority; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

var (
	ErrFallbackPolicyNoID           = &FallbackPolicyError{"policy id is required"}
	ErrFallbackPolicyNoName         = &FallbackPolicyError{"policy name is required"}
	ErrFallbackPolicyInvalidStrategy = &FallbackPolicyError{"invalid strategy type"}
	ErrFallbackPolicyNoRules        = &FallbackPolicyError{"at least one rule is required"}
	ErrFallbackRuleNoBackend        = &FallbackPolicyError{"rule backend_id is required"}
	ErrFallbackRuleNoModel          = &FallbackPolicyError{"rule model is required"}
	ErrFallbackPolicyNotFound       = &FallbackPolicyError{"fallback policy not found"}
)

type FallbackPolicyError struct {
	Message string
}

func (e *FallbackPolicyError) Error() string {
	return e.Message
}
