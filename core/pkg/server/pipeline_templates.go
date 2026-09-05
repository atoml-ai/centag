package server

import (
	"strings"

	"centag/core/pkg/bootstrap"
	"centag/core/pkg/configsync"
	"centag/core/pkg/logger"
	"centag/core/pkg/pipeline"
)

// resolvePipelineTemplates returns pipeline templates for the current product edition
// (CENTAG_EDITION / cfg.Server.Edition). Empty edition loads common+team.
func resolvePipelineTemplates() []pipeline.PatternTemplate {
	return resolvePipelineTemplatesWithEdition("")
}

// resolvePipelineTemplatesWithEdition 按 edition 子目录加载：
//
//	minimal / personal → pipeline-templates/common/
//	team               → common/ + team/
//
// 在内置 initdata 模板之上叠加远端同步模板（configsync → pipeline_templates 表）。
// 同 ID 时远端覆盖本地：飞书线上表格是持续更新的数据源，随包分发的 initdata
// 只是离线兜底快照。
func resolvePipelineTemplatesWithEdition(edition string) []pipeline.PatternTemplate {
	initialTemplates := bootstrap.LoadInitialPipelineTemplatesWithEdition(edition)
	templates := convertInitialTemplates(initialTemplates)

	templates = mergeRemotePipelineTemplates(templates, edition)

	// 无 initdata 模板时返回空列表：首页/列表显示空白，不注入 simple-chat/chat-node 兜底。
	if len(templates) == 0 {
		logger.Warnf("no pipeline templates loaded for edition=%q; store will stay empty until import/create", edition)
	}

	return templates
}

// mergeRemotePipelineTemplates overlays remote-synced pipeline templates
// (configsync → pipeline_templates DB table) on top of the bundled initdata
// templates. Remote templates win on ID conflicts.
func mergeRemotePipelineTemplates(templates []pipeline.PatternTemplate, edition string) []pipeline.PatternTemplate {
	store, err := pipeline.NewDBPipelineTemplateStore()
	if err != nil {
		// DB unavailable — fall back to bundled templates only.
		return templates
	}
	var remote []configsync.PipelineTemplate
	if edition == "" {
		remote, err = store.ListAll()
	} else {
		remote, err = store.ListByEdition(edition)
	}
	if err != nil || len(remote) == 0 {
		return templates
	}

	byID := make(map[string]int, len(templates))
	for i := range templates {
		byID[templates[i].ID] = i
	}
	overridden := 0
	for _, rt := range remote {
		converted := convertConfigsyncTemplate(rt)
		if converted == nil {
			continue
		}
		if idx, ok := byID[converted.ID]; ok {
			templates[idx] = *converted
			overridden++
			continue
		}
		byID[converted.ID] = len(templates)
		templates = append(templates, *converted)
	}
	if overridden > 0 {
		logger.Infof("pipeline templates: %d remote template(s) override bundled initdata (edition=%q)", overridden, edition)
	}
	return templates
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
						FallbackNodes: fg.FallbackNodes,
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

// convertConfigsyncTemplate converts a remote-synced pipeline template
// (configsync.PipelineTemplate, as persisted in the pipeline_templates table)
// into a pipeline.PatternTemplate. Field mapping mirrors
// convertInitialTemplates — the configsync types are structurally identical
// to the bootstrap.Initial* types.
func convertConfigsyncTemplate(tmpl configsync.PipelineTemplate) *pipeline.PatternTemplate {
	id := strings.TrimSpace(tmpl.ID)
	if id == "" {
		return nil
	}

	converted := &pipeline.PatternTemplate{
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
		if len(tmpl.GlobalConfig.FallbackGroups) > 0 {
			converted.GlobalConfig.FallbackGroups = make([]pipeline.FallbackGroup, 0, len(tmpl.GlobalConfig.FallbackGroups))
			for _, fg := range tmpl.GlobalConfig.FallbackGroups {
				converted.GlobalConfig.FallbackGroups = append(converted.GlobalConfig.FallbackGroups, pipeline.FallbackGroup{
					PrimaryNodeID: fg.PrimaryNodeID,
					FallbackNodes: fg.FallbackNodes,
					MaxAttempts:   fg.MaxAttempts,
				})
			}
		}
		if tmpl.GlobalConfig.Storage != nil {
			converted.GlobalConfig.StorageConfig = &pipeline.StorageHookConfig{
				Enabled:       tmpl.GlobalConfig.Storage.Enabled,
				Namespace:     tmpl.GlobalConfig.Storage.Namespace,
				AutoSave:      tmpl.GlobalConfig.Storage.AutoSave,
				SaveInterval:  tmpl.GlobalConfig.Storage.SaveInterval,
				RetentionDays: tmpl.GlobalConfig.Storage.RetentionDays,
			}
		}
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

	return converted
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
