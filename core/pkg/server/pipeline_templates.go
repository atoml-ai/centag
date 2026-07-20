package server

import (
	"strings"

	"centag/core/pkg/bootstrap"
	"centag/core/pkg/logger"
	"centag/core/pkg/pipeline"
)

// resolvePipelineTemplates returns pipeline templates loaded from config/initdata/pipeline-templates/ YAML files.
// Returns nil if no templates are found (system will use DB-stored pipelines instead).
func resolvePipelineTemplates() []pipeline.PatternTemplate {
	return resolvePipelineTemplatesWithEdition("")
}

// resolvePipelineTemplatesWithEdition 根据版本返回流水线模板。
// edition 为空时加载所有模板（向后兼容），否则根据文件名前缀过滤：
//   - "minimal-" 前缀：仅 minimal 版加载
//   - "personal-" 前缀：personal 和 team 版加载
//   - "all-" 前缀：所有版本加载
//   - 无前缀：所有版本加载（向后兼容）
func resolvePipelineTemplatesWithEdition(edition string) []pipeline.PatternTemplate {
	initialTemplates := bootstrap.LoadInitialPipelineTemplatesWithEdition(edition)
	templates := convertInitialTemplates(initialTemplates)

	// 文件加载失败时使用内置兜底模板
	if len(templates) == 0 {
		templates = defaultBuiltinTemplates()
	}

	return templates
}

// defaultBuiltinTemplates 返回内置兜底模板，确保启动后始终有模板可用
func defaultBuiltinTemplates() []pipeline.PatternTemplate {
	return []pipeline.PatternTemplate{
		{
			ID:          "simple-chat",
			Name:        "Simple Chat",
			Description: "Single generator node for basic chat",
			ShortcutCode: "#chat",
			Metadata:    map[string]interface{}{"builtin": true},
			GlobalConfig: &pipeline.GlobalPipelineConfig{
				Timeout:       120,
				MaxRetries:    3,
				BypassOnError: true,
				ParallelLimit: 4,
			},
			Nodes: []pipeline.PipelineNodeConfig{
				{
					ID:   "chat-node",
					Type: pipeline.NodeTypeGenerator,
					Name: "Chat Generator",
					Config: pipeline.NodeConfig{
						PromptTemplate: "{{input}}",
						SystemPrompt:   "You are a helpful assistant.",
					},
				},
			},
		},
	}
}

func convertInitialTemplates(initial []bootstrap.InitialPipelineTemplate) []pipeline.PatternTemplate {
	if len(initial) == 0 {
		return nil
	}

	templates := make([]pipeline.PatternTemplate, 0, len(initial))
	for _, tmpl := range initial {
		id := strings.TrimSpace(tmpl.ID)
		if id == "" {
			continue
		}

		converted := pipeline.PatternTemplate{
			ID:            id,
			SchemaVersion: tmpl.SchemaVersion,
			Name:          tmpl.Name,
			Description:   tmpl.Description,
			ShortcutCode:  tmpl.ShortcutCode,
			Metadata:      tmpl.Metadata,
			Nodes:         make([]pipeline.PipelineNodeConfig, 0, len(tmpl.Nodes)),
		}

		if tmpl.GlobalConfig != nil {
			converted.GlobalConfig = &pipeline.GlobalPipelineConfig{
				Timeout:       tmpl.GlobalConfig.Timeout,
				MaxRetries:    tmpl.GlobalConfig.MaxRetries,
				BypassOnError: tmpl.GlobalConfig.BypassOnError,
				ParallelLimit: tmpl.GlobalConfig.ParallelLimit,
				LogLevel:      tmpl.GlobalConfig.LogLevel,
				SystemPrompt:  tmpl.GlobalConfig.SystemPrompt,
			}
			// 转换 FallbackGroups
			if len(tmpl.GlobalConfig.FallbackGroups) > 0 {
				converted.GlobalConfig.FallbackGroups = make([]pipeline.FallbackGroup, 0, len(tmpl.GlobalConfig.FallbackGroups))
				for _, fg := range tmpl.GlobalConfig.FallbackGroups {
					converted.GlobalConfig.FallbackGroups = append(converted.GlobalConfig.FallbackGroups, pipeline.FallbackGroup{
						PrimaryNodeID: fg.PrimaryNodeID,
						FallbackNodes:  fg.FallbackNodes,
						MaxAttempts:   fg.MaxAttempts,
					})
				}
			}
			// 转换 Storage 存储钩子配置
			if tmpl.GlobalConfig.Storage != nil {
				converted.GlobalConfig.StorageConfig = &pipeline.StorageHookConfig{
					Enabled:       tmpl.GlobalConfig.Storage.Enabled,
					Namespace:     tmpl.GlobalConfig.Storage.Namespace,
					AutoSave:      tmpl.GlobalConfig.Storage.AutoSave,
					SaveInterval:  tmpl.GlobalConfig.Storage.SaveInterval,
					RetentionDays: tmpl.GlobalConfig.Storage.RetentionDays,
				}
			}
			// 转换 Hooks 钩子配置
			if len(tmpl.GlobalConfig.Hooks) > 0 {
				converted.GlobalConfig.Hooks = make([]pipeline.HookConfig, 0, len(tmpl.GlobalConfig.Hooks))
				for _, h := range tmpl.GlobalConfig.Hooks {
					hook := pipeline.HookConfig{
						Type:        h.Type,
						On:          append([]string{}, h.On...),
						StorageName: h.StorageName,
					}
					if len(h.Config) > 0 {
						hook.Config = make(map[string]interface{}, len(h.Config))
						for k, v := range h.Config {
							hook.Config[k] = v
						}
					}
					converted.GlobalConfig.Hooks = append(converted.GlobalConfig.Hooks, hook)
				}
			}
		}

		for _, node := range tmpl.Nodes {
			pn := pipeline.PipelineNodeConfig{
				ID:             node.ID,
				Type:           pipeline.NodeType(node.Type),
				Kind:           node.Kind,
				Implementation: node.Implementation,
				Name:           node.Name,
				Backend:        node.Backend,
				Model:          node.Model,
				Config: pipeline.NodeConfig{
					Backend:        node.Config.Backend,
					Model:          node.Config.Model,
					PromptTemplate: node.Config.PromptTemplate,
					SystemPrompt:   node.Config.SystemPrompt,
					Temperature:    node.Config.Temperature,
					MaxTokens:      node.Config.MaxTokens,
					CustomConfig:   node.Config.CustomConfig,
					TemplateVars:   node.Config.TemplateVars,
				},
				Inputs:          node.Inputs,
				Outputs:         node.Outputs,
				ConfigSchemaRef: node.ConfigSchemaRef,
				SecretsRef:      node.SecretsRef,
				Permissions:     node.Permissions,
				Timeout:         node.Timeout,
				Condition:       node.Condition,
				NextNodes:       node.NextNodes,
				DependsOn:       node.DependsOn,
			}
			if node.Retry != nil {
				pn.Retry = &pipeline.RetryConfig{
					MaxAttempts:     node.Retry.MaxAttempts,
					BackoffStrategy: node.Retry.BackoffStrategy,
					InitialDelay:    node.Retry.InitialDelay,
					MaxDelay:        node.Retry.MaxDelay,
				}
			}
			if node.RouteConfig != nil {
				pn.RouteConfig = &pipeline.RouteConfig{
					RouterNodeID: node.RouteConfig.RouterNodeID,
					RouteValue:   node.RouteConfig.RouteValue,
					IsDefault:    node.RouteConfig.IsDefault,
				}
			}
			// 归一化：将顶层 Backend/Model 归入 Config，统一出口
			pn.Normalize()
			converted.Nodes = append(converted.Nodes, pn)
		}

		templates = append(templates, converted)
	}

	return templates
}

// backfillRouteConfigForExistingPipelines 为已有流水线回填 RouteConfig。
//
// 在首次启动之后创建的流水线由于 Template 加载阶段的 bug（InitialPipelineNodeConfig
// 缺少 RouteConfig 字段），其节点的 RouteConfig 全部为 nil，导致路由模式无法正确过滤分支。
// 本函数检测每个流水线节点，如果 RouteConfig 为 nil 但同 ID 的模板节点有 RouteConfig，
// 则从模板复制过来，并持久化到 store。
// 注意：必须扫描全局和租户两类流水线，因为 tenant pipeline 不在 List() 的返回中。
func backfillRouteConfigForExistingPipelines(registry *pipeline.PipelineRegistry, templates []pipeline.PatternTemplate) int {
	updated := 0

	// 收集所有流水线（全局 + 所有租户）
	seen := make(map[*pipeline.AgentPatternPipeline]bool)
	for _, p := range registry.List() {
		seen[p] = true
	}
	for _, p := range registry.ListAll() {
		seen[p] = true
	}

	for p := range seen {
		var tmpl *pipeline.PatternTemplate
		for i := range templates {
			if templates[i].ID == p.ID {
				tmpl = &templates[i]
				break
			}
		}
		if tmpl == nil {
			continue
		}

		modified := false
		for ni := range p.Nodes {
			if p.Nodes[ni].RouteConfig != nil {
				continue
			}
			for _, tmplNode := range tmpl.Nodes {
				if tmplNode.ID == p.Nodes[ni].ID && tmplNode.RouteConfig != nil {
					rc := *tmplNode.RouteConfig
					p.Nodes[ni].RouteConfig = &rc
					modified = true
					updated++
					break
				}
			}
		}

		if modified {
			if err := registry.Register(p); err != nil {
				logger.Warnf("Failed to persist pipeline %s after RouteConfig backfill: %v", p.ID, err)
			}
		}
	}

	return updated
}

// injectRouteConfigFromTemplate 将模板中的 RouteConfig 注入到流水线的 API 响应中。
// 如果流水线节点缺少 RouteConfig 但同 ID 的模板节点有，则注入。
// 返回克隆后的流水线，不修改原始对象。
func injectRouteConfigFromTemplate(p *pipeline.AgentPatternPipeline, templates []pipeline.PatternTemplate) *pipeline.AgentPatternPipeline {
	var tmpl *pipeline.PatternTemplate
	for i := range templates {
		if templates[i].ID == p.ID {
			tmpl = &templates[i]
			break
		}
	}
	if tmpl == nil {
		return p
	}

	needClone := false
	for _, node := range p.Nodes {
		if node.RouteConfig == nil {
			for _, tmplNode := range tmpl.Nodes {
				if tmplNode.ID == node.ID && tmplNode.RouteConfig != nil {
					needClone = true
					break
				}
			}
		}
		if needClone {
			break
		}
	}
	if !needClone {
		return p
	}

	// 克隆流水线，避免修改注册表中的原始对象
	clone := *p
	clone.Nodes = make([]pipeline.PipelineNodeConfig, len(p.Nodes))
	for i := range p.Nodes {
		clone.Nodes[i] = p.Nodes[i]
		if clone.Nodes[i].RouteConfig == nil {
			for _, tmplNode := range tmpl.Nodes {
				if tmplNode.ID == clone.Nodes[i].ID && tmplNode.RouteConfig != nil {
					rc := *tmplNode.RouteConfig
					clone.Nodes[i].RouteConfig = &rc
					break
				}
			}
		}
	}
	return &clone
}
