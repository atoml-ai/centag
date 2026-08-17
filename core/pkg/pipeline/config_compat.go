package pipeline

import (
	"fmt"
	"strings"
)

// ConfigCompatLayer 配置兼容层
// 自动转换旧配置到新格式，保持向后兼容
type ConfigCompatLayer struct{}

// NewConfigCompatLayer 创建配置兼容层
func NewConfigCompatLayer() *ConfigCompatLayer {
	return &ConfigCompatLayer{}
}

// ConvertPipelineConfig 转换旧流水线配置到新格式
func (c *ConfigCompatLayer) ConvertPipelineConfig(oldConfig map[string]interface{}) map[string]interface{} {
	newConfig := make(map[string]interface{})

	// 复制所有字段
	for k, v := range oldConfig {
		newConfig[k] = v
	}

	// 检测并转换旧格式
	if id, ok := oldConfig["id"].(string); ok {
		switch id {
		case "transparent-proxy", "direct-backend", "fixed-egress", "transparent-passthrough":
			// 透传类流水线统一为 transparent
			newConfig["id"] = "transparent"
			newConfig["name"] = "透明模式"
			newConfig["description"] = "简单直接的透传转发流水线，支持透明代理、直连后端、固定出口等多种模式"
			newConfig["shortcut_code"] = "#t"
			newConfig = c.convertTransparentPassthrough(newConfig, oldConfig)
		case "router-mode":
			// 路由类流水线统一为 router-pipeline
			newConfig["id"] = "router-pipeline"
			newConfig["name"] = "Router Pipeline"
			newConfig["description"] = "通用路由流水线，支持关键词匹配、LLM分类、混合路由等多种策略"
			newConfig["shortcut_code"] = "#r"
			newConfig = c.convertRouterPipeline(newConfig, oldConfig)
		case "agent-skill-router":
			// agent-skill-router 改名为 centag-ops-router
			newConfig["id"] = "centag-ops-router"
			newConfig["name"] = "Centag Ops Router"
			newConfig["description"] = "Centag运维技能路由：状态检查、配置分析、错误诊断、日志分析、策略建议"
			newConfig["shortcut_code"] = "#ops"
			newConfig = c.convertCentagOpsRouter(newConfig, oldConfig)
		case "cache-hit", "cache-mode", "18-rag-mode":
			// 缓存类流水线统一为 cache-pipeline
			newConfig["id"] = "cache-pipeline"
			newConfig["name"] = "Cache Pipeline"
			newConfig["description"] = "统一缓存流水线，支持缓存读写、RAG检索增强生成等多种模式"
			newConfig["shortcut_code"] = "#cache"
			newConfig = c.convertCachePipeline(newConfig, oldConfig)
		}
	}

	return newConfig
}

// convertTransparentPassthrough 转换透传类流水线配置
func (c *ConfigCompatLayer) convertTransparentPassthrough(newConfig, oldConfig map[string]interface{}) map[string]interface{} {
	// 保留旧的 metadata
	if metadata, ok := oldConfig["metadata"].(map[string]interface{}); ok {
		newConfig["metadata"] = metadata
	} else {
		newConfig["metadata"] = make(map[string]interface{})
	}

	// 设置模式映射
	metadata := newConfig["metadata"].(map[string]interface{})
	modeMappings := make(map[string]interface{})

	// 从旧配置提取模式参数
	if nodes, ok := oldConfig["nodes"].([]interface{}); ok && len(nodes) > 0 {
		if firstNode, ok := nodes[0].(map[string]interface{}); ok {
			if config, ok := firstNode["config"].(map[string]interface{}); ok {
				if customConfig, ok := config["custom_config"].(map[string]interface{}); ok {
					// 提取 route_policy 和 inject_system_prompt
					routePolicy := "match_model"
					if rp, ok := customConfig["route_policy"].(string); ok {
						routePolicy = rp
					}

					injectSystemPrompt := false
					if isp, ok := customConfig["inject_system_prompt"].(bool); ok {
						injectSystemPrompt = isp
					}

					systemPromptStrategy := "passthrough"
					if sps, ok := customConfig["system_prompt_strategy"].(string); ok {
						systemPromptStrategy = sps
					}

					// 设置模式映射
					modeMappings["transparent-proxy"] = map[string]interface{}{
						"route_policy":           "match_model",
						"inject_system_prompt":   false,
						"system_prompt_strategy": "passthrough",
					}
					modeMappings["direct-backend"] = map[string]interface{}{
						"route_policy":           "fixed",
						"inject_system_prompt":   true,
						"system_prompt_strategy": "replace",
					}
					modeMappings["fixed-egress"] = map[string]interface{}{
						"route_policy":           "fixed",
						"inject_system_prompt":   false,
						"system_prompt_strategy": "passthrough",
					}

					// 设置默认参数
					metadata["default_route_policy"] = routePolicy
					metadata["default_inject_system_prompt"] = injectSystemPrompt
					metadata["default_system_prompt_strategy"] = systemPromptStrategy
				}
			}
		}
	}

	metadata["mode_mappings"] = modeMappings
	metadata["aligned_proxy_mode"] = "transparent"

	// 保留原始ID对应的快捷方式
	originalID, _ := oldConfig["id"].(string)
	if originalShortcut, ok := oldConfig["shortcut_code"].(string); ok {
		metadata[originalID+"_shortcut"] = originalShortcut
	}

	return newConfig
}

// convertRouterPipeline 转换路由类流水线配置
func (c *ConfigCompatLayer) convertRouterPipeline(newConfig, oldConfig map[string]interface{}) map[string]interface{} {
	// 保留旧的 metadata
	if metadata, ok := oldConfig["metadata"].(map[string]interface{}); ok {
		newConfig["metadata"] = metadata
	} else {
		newConfig["metadata"] = make(map[string]interface{})
	}

	// 设置策略映射
	metadata := newConfig["metadata"].(map[string]interface{})
	strategyMappings := make(map[string]interface{})

	// 从旧配置提取策略参数
	if nodes, ok := oldConfig["nodes"].([]interface{}); ok && len(nodes) > 0 {
		if firstNode, ok := nodes[0].(map[string]interface{}); ok {
			if config, ok := firstNode["config"].(map[string]interface{}); ok {
				if customConfig, ok := config["custom_config"].(map[string]interface{}); ok {
					// 提取 routing_strategy
					routingStrategy := "keyword_contains"
					if rs, ok := customConfig["routing_strategy"].(string); ok {
						routingStrategy = rs
					}

					// 设置策略映射
					strategyMappings["router-mode"] = map[string]interface{}{
						"routing_strategy": "keyword_contains",
					}
					strategyMappings["agent-skill-router"] = map[string]interface{}{
						"routing_strategy": "llm_classify",
					}

					// 设置默认策略
					metadata["default_routing_strategy"] = routingStrategy
				}
			}
		}
	}

	metadata["strategy_mappings"] = strategyMappings
	metadata["router_pipeline"] = true

	// 保留原始ID对应的快捷方式
	originalID, _ := oldConfig["id"].(string)
	if originalShortcut, ok := oldConfig["shortcut_code"].(string); ok {
		metadata[originalID+"_shortcut"] = originalShortcut
	}

	return newConfig
}

// convertCentagOpsRouter 转换Centag运维路由配置
func (c *ConfigCompatLayer) convertCentagOpsRouter(newConfig, oldConfig map[string]interface{}) map[string]interface{} {
	// 保留旧的 metadata
	if metadata, ok := oldConfig["metadata"].(map[string]interface{}); ok {
		newConfig["metadata"] = metadata
	} else {
		newConfig["metadata"] = make(map[string]interface{})
	}

	// 更新 metadata
	metadata := newConfig["metadata"].(map[string]interface{})
	metadata["centag_ops_pipeline"] = true
	metadata["centag_ops_router"] = true

	// 移除旧的 centag_ops_pipeline（如果存在）
	delete(metadata, "skill_router")

	// 保留原始ID对应的快捷方式
	originalID, _ := oldConfig["id"].(string)
	if originalShortcut, ok := oldConfig["shortcut_code"].(string); ok {
		metadata[originalID+"_shortcut"] = originalShortcut
	}

	return newConfig
}

// convertCachePipeline 转换缓存类流水线配置
func (c *ConfigCompatLayer) convertCachePipeline(newConfig, oldConfig map[string]interface{}) map[string]interface{} {
	// 保留旧的 metadata
	if metadata, ok := oldConfig["metadata"].(map[string]interface{}); ok {
		newConfig["metadata"] = metadata
	} else {
		newConfig["metadata"] = make(map[string]interface{})
	}

	// 设置缓存配置
	metadata := newConfig["metadata"].(map[string]interface{})
	cacheConfig := make(map[string]interface{})

	// 从旧配置提取缓存参数
	if nodes, ok := oldConfig["nodes"].([]interface{}); ok {
		for _, node := range nodes {
			if nodeMap, ok := node.(map[string]interface{}); ok {
				if nodeType, ok := nodeMap["type"].(string); ok && nodeType == "cache" {
					if config, ok := nodeMap["config"].(map[string]interface{}); ok {
						if customConfig, ok := config["custom_config"].(map[string]interface{}); ok {
							// 提取缓存配置
							if operation, ok := customConfig["operation"].(string); ok {
								if operation == "read" {
									cacheConfig["enable_cache_read"] = true
								} else if operation == "write" {
									cacheConfig["enable_cache_write"] = true
								}
							}
							if strategy, ok := customConfig["strategy"].(string); ok {
								cacheConfig["cache_strategy"] = strategy
							}
						}
					}
				}
			}
		}
	}

	// 设置默认缓存配置
	if _, ok := cacheConfig["enable_cache_read"]; !ok {
		cacheConfig["enable_cache_read"] = true
	}
	if _, ok := cacheConfig["enable_cache_write"]; !ok {
		cacheConfig["enable_cache_write"] = true
	}
	if _, ok := cacheConfig["cache_strategy"]; !ok {
		cacheConfig["cache_strategy"] = "exact"
	}

	metadata["cache_config"] = cacheConfig
	metadata["cache_pipeline"] = true

	// 保留原始ID对应的快捷方式
	originalID, _ := oldConfig["id"].(string)
	if originalShortcut, ok := oldConfig["shortcut_code"].(string); ok {
		metadata[originalID+"_shortcut"] = originalShortcut
	}

	return newConfig
}

// ConvertNodeConfig 转换旧节点配置到新格式
func (c *ConfigCompatLayer) ConvertNodeConfig(oldConfig map[string]interface{}) map[string]interface{} {
	newConfig := make(map[string]interface{})

	// 复制所有字段
	for k, v := range oldConfig {
		newConfig[k] = v
	}

	// 检测并转换旧格式
	if impl, ok := oldConfig["implementation"].(string); ok {
		switch impl {
		case "builtin.generator":
			// GeneratorNode 保持不变，但标记为 deprecated
			if metadata, ok := newConfig["metadata"].(map[string]interface{}); ok {
				metadata["deprecated"] = true
				metadata["deprecated_reason"] = "请使用 builtin.llm_call 替代"
			} else {
				newConfig["metadata"] = map[string]interface{}{
					"deprecated":        true,
					"deprecated_reason": "请使用 builtin.llm_call 替代",
				}
			}
		case "builtin.processor":
			// ProcessorNode 已废弃，建议使用 builtin.llm_call
			if metadata, ok := newConfig["metadata"].(map[string]interface{}); ok {
				metadata["deprecated"] = true
				metadata["deprecated_reason"] = "已废弃，请使用 builtin.llm_call 替代"
			} else {
				newConfig["metadata"] = map[string]interface{}{
					"deprecated":        true,
					"deprecated_reason": "已废弃，请使用 builtin.llm_call 替代",
				}
			}
		}
	}

	return newConfig
}

// IsLegacyPipeline 检查是否为旧版流水线配置
func (c *ConfigCompatLayer) IsLegacyPipeline(config map[string]interface{}) bool {
	id, _ := config["id"].(string)
	legacyIDs := []string{
		"transparent-proxy",
		"direct-backend",
		"fixed-egress",
		"transparent-passthrough",
		"router-mode",
		"agent-skill-router",
		"cache-hit",
		"cache-mode",
		"18-rag-mode",
	}

	for _, legacyID := range legacyIDs {
		if id == legacyID {
			return true
		}
	}

	return false
}

// GetActualPipelineID 获取实际流水线ID（处理别名）
func (c *ConfigCompatLayer) GetActualPipelineID(requestedID string) string {
	aliases := map[string]string{
		"transparent-proxy":      "transparent",
		"direct-backend":         "transparent",
		"fixed-egress":           "transparent",
		"transparent-passthrough": "transparent",
		"router-mode":            "router-pipeline",
		"agent-skill-router":     "centag-ops-router",
		"cache-hit":              "cache-pipeline",
		"cache-mode":             "cache-pipeline",
		"18-rag-mode":            "cache-pipeline",
	}

	if actualID, ok := aliases[requestedID]; ok {
		return actualID
	}

	return requestedID
}

// GetActualShortcutCode 获取实际快捷方式（处理别名）
func (c *ConfigCompatLayer) GetActualShortcutCode(requestedCode string) string {
	aliases := map[string]string{
		"#skill": "#ops",
	}

	if actualCode, ok := aliases[requestedCode]; ok {
		return actualCode
	}

	return requestedCode
}

// ValidateLegacyConfig 验证旧版配置兼容性
func (c *ConfigCompatLayer) ValidateLegacyConfig(config map[string]interface{}) []string {
	var warnings []string

	id, _ := config["id"].(string)

	// 检查是否为旧版流水线
	if c.IsLegacyPipeline(config) {
		warnings = append(warnings, fmt.Sprintf("流水线 '%s' 使用旧版ID，已自动转换为新版格式", id))
	}

	// 检查是否使用旧版快捷方式
	if shortcut, ok := config["shortcut_code"].(string); ok && shortcut == "#skill" {
		warnings = append(warnings, "快捷方式 '#skill' 已废弃，建议使用 '#ops'")
	}

	// 检查是否使用旧版 metadata
	if metadata, ok := config["metadata"].(map[string]interface{}); ok {
		if _, ok := metadata["agent_skill_pipeline"]; ok {
			warnings = append(warnings, "metadata 'agent_skill_pipeline' 已废弃，建议使用 'centag_ops_pipeline'")
		}
		if _, ok := metadata["skill_router"]; ok {
			warnings = append(warnings, "metadata 'skill_router' 已废弃，建议使用 'centag_ops_router'")
		}
	}

	return warnings
}

// ConvertAllLegacyConfigs 批量转换旧版配置
func (c *ConfigCompatLayer) ConvertAllLegacyConfigs(configs []map[string]interface{}) []map[string]interface{} {
	var convertedConfigs []map[string]interface{}

	for _, config := range configs {
		if c.IsLegacyPipeline(config) {
			convertedConfig := c.ConvertPipelineConfig(config)
			convertedConfigs = append(convertedConfigs, convertedConfig)
		} else {
			convertedConfigs = append(convertedConfigs, config)
		}
	}

	return convertedConfigs
}

// GetPipelineType 获取流水线类型
func (c *ConfigCompatLayer) GetPipelineType(config map[string]interface{}) string {
	id, _ := config["id"].(string)

	if strings.HasPrefix(id, "transparent") || id == "direct-backend" || id == "fixed-egress" {
		return "transparent"
	}

	if strings.HasPrefix(id, "router") || id == "agent-skill-router" || id == "centag-ops-router" {
		return "router"
	}

	if strings.HasPrefix(id, "cache") || id == "18-rag-mode" {
		return "cache"
	}

	return "unknown"
}