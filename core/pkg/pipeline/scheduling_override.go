package pipeline

import "strings"

// applySchedulingOverrides applies backend/model chosen by an upstream scheduler node.
func applySchedulingOverrides(nodeConfig *NodeConfig, input *NodeInput, execCtx *ExecutionContext) {
	if nodeConfig == nil {
		return
	}

	// fallback 节点必须使用自身的 {{system.fallback_backend}} / {{system.fallback_model}}，
	// 不能被上游 scheduler 的调度决策覆盖，否则降级会与主节点打同一后端/模型。
	if isFallbackNodeConfig(nodeConfig) {
		return
	}

	backendID := ""
	model := ""

	if input != nil {
		if input.Metadata != nil {
			if v, ok := input.Metadata["backend_id"].(string); ok && v != "" {
				backendID = v
			}
			if v, ok := input.Metadata["model"].(string); ok && v != "" {
				model = v
			}
		}
		for _, result := range input.UpstreamResults {
			if result == nil || result.Metadata == nil {
				continue
			}
			scheduled, _ := result.Metadata["scheduler_decision"].(bool)
			geo, _ := result.Metadata["geo_decision"].(bool)
			if !scheduled && !geo {
				continue
			}
			if v, ok := result.Metadata["backend_id"].(string); ok && v != "" {
				backendID = v
			}
			if v, ok := result.Metadata["model"].(string); ok && v != "" {
				model = v
			}
		}
	}

	if execCtx != nil {
		if v, ok := execCtx.GetVariable("backend_id"); ok {
			if s, ok := v.(string); ok && s != "" {
				backendID = s
			}
		}
		if v, ok := execCtx.GetVariable("scheduled_model"); ok {
			if s, ok := v.(string); ok && s != "" {
				model = s
			}
		}
	}

	if backendID != "" {
		nodeConfig.Backend = backendID
	}
	if model != "" {
		nodeConfig.Model = model
	}
}

// isFallbackNodeConfig 判断节点是否为降级节点。
// 命中任一条件即视为降级节点：
//  1. config.custom_config.is_fallback 为 true（transparent 流水线的显式标记）；
//  2. backend/model 引用 {{system.fallback_backend}} / {{system.fallback_model}}（smart-scheduling 的降级节点）。
func isFallbackNodeConfig(nodeConfig *NodeConfig) bool {
	if nodeConfig == nil {
		return false
	}

	if nodeConfig.CustomConfig != nil {
		if v, ok := nodeConfig.CustomConfig["is_fallback"]; ok {
			switch t := v.(type) {
			case bool:
				if t {
					return true
				}
			case string:
				if strings.EqualFold(strings.TrimSpace(t), "true") {
					return true
				}
			}
		}
	}

	if strings.Contains(nodeConfig.Backend, "{{system.fallback_backend}}") ||
		strings.Contains(nodeConfig.Model, "{{system.fallback_model}}") {
		return true
	}

	return false
}