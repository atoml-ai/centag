package pipeline

import (
	"fmt"
	"time"

	"centag/core/pkg/config"
)

// MigrateFallbackGroupsToPolicy 将流水线的 fallback_groups 转换为 GlobalFallbackPolicy。
// 调用时机：流水线保存时，若存在 fallback_groups 且无 fallback_policy_id，自动迁移。
func MigrateFallbackGroupsToPolicy(pipelineID string, fallbackGroups []FallbackGroup) *config.GlobalFallbackPolicy {
	if len(fallbackGroups) == 0 {
		return nil
	}

	// 取第一个 group（简化迁移逻辑）
	fg := fallbackGroups[0]

	// 构建降级规则：自定义链策略
	rules := make([]config.FallbackRule, 0, len(fg.FallbackNodes)+1)

	// 主节点作为第一优先级
	rules = append(rules, config.FallbackRule{
		Priority:  1,
		BackendID: fmt.Sprintf("__node_%s_backend", fg.PrimaryNodeID),
		Model:     fmt.Sprintf("__node_%s_model", fg.PrimaryNodeID),
	})

	// 备用节点作为后续优先级
	for i, fbID := range fg.FallbackNodes {
		rules = append(rules, config.FallbackRule{
			Priority:  i + 2,
			BackendID: fmt.Sprintf("__node_%s_backend", fbID),
			Model:     fmt.Sprintf("__node_%s_model", fbID),
		})
	}

	policy := &config.GlobalFallbackPolicy{
		ID:          fmt.Sprintf("__migrated_%s", pipelineID),
		Name:        fmt.Sprintf("从降级组迁移 (%s)", pipelineID),
		Description: fmt.Sprintf("自动从流水线 %s 的 fallback_groups 迁移而来", pipelineID),
		Strategy:    config.StrategyCustomChain,
		Rules:       rules,
		Enabled:     true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	return policy
}

// NeedsMigration 判断流水线是否需要迁移 fallback_groups。
func NeedsMigration(fallbackGroups []FallbackGroup, fallbackPolicyID string) bool {
	return len(fallbackGroups) > 0 && fallbackPolicyID == ""
}
