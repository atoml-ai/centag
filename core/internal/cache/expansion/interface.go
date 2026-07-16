package expansion

import (
	"context"

	"centag/core/pkg/plugin"
)

// Expander 查询展开器接口
type Expander interface {
	// Expand 将当前查询展开为完整、独立的查询
	//
	// 参数:
	//   - ctx: 上下文
	//   - current: 当前用户消息
	//   - history: 历史消息（最近N轮）
	//
	// 返回:
	//   - expanded: 展开后的查询（如果无需展开则返回原查询）
	//   - isExpanded: 是否进行了展开
	//   - err: 错误
	Expand(
		ctx context.Context,
		current string,
		history []plugin.Message,
	) (expanded string, isExpanded bool, err error)

	// Name 返回展开器名称
	Name() string
}

// Config 展开器配置
type Config struct {
	Mode string `json:"mode" yaml:"mode" default:"rule"` // rule | llm | hybrid | none

	RuleBased RuleConfig `json:"rule_based" yaml:"rule_based"`
	LLMBased  LLMConfig  `json:"llm_based" yaml:"llm_based"`
}

// RuleConfig 规则展开配置
type RuleConfig struct {
	Enabled          bool     `json:"enabled" yaml:"enabled" default:"true"`
	MaxHistoryRounds int      `json:"max_history_rounds" yaml:"max_history_rounds" default:"3"`
	Pronouns         []string `json:"pronouns" yaml:"pronouns"`
	MinEntityScore   float64  `json:"min_entity_score" yaml:"min_entity_score" default:"0.5"`
}

// LLMConfig LLM展开配置
type LLMConfig struct {
	Enabled   bool   `json:"enabled" yaml:"enabled" default:"false"`
	Timeout   int    `json:"timeout_ms" yaml:"timeout_ms" default:"500"`
	Model     string `json:"model" yaml:"model" default:"gpt-3.5-turbo"`
	MaxTokens int    `json:"max_tokens" yaml:"max_tokens" default:"100"`
}
