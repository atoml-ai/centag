package server

import (
	"fmt"

	"centag/core/internal/agent/skills"
	"centag/core/pkg/logger"
	"centag/core/pkg/pipeline"
)

// skillPipelineSchemaVersion skill router pipeline 的 schema 版本（与内置模板一致）。
const skillPipelineSchemaVersion = "centag.pipeline/v1alpha1"

// skillPipelineMetadataKey 标记该 pipeline 由 skill 插件自动生成。
const skillPipelineMetadataKey = "centag_ops_pipeline"

// centagOpsRouterPipelineID 所有 skill 共用的单一路由管线 id。
// 执行时通过 X-Pipeline-ID 选中该管线，由 router 节点对用户问题做 LLM 分类，
// 自动路由到对应 skill 分支（技术方案「单一路由 + skill 分支」模型）。
const centagOpsRouterPipelineID = "centag-ops-router"

// agentSkillRouterClassifierID router 管线中的意图分类节点 id。
const agentSkillRouterClassifierID = "skill-classifier"

// agentSkillRouterChatID 未命中任何 skill 时的默认 chat 分支节点 id。
const agentSkillRouterChatID = "chat-gen"

// skillBranchNodeID 由 skill 注册名推导分支 generator 节点 id。
// status-check → status-check-gen
func skillBranchNodeID(skillName string) string {
	return skillName + "-gen"
}

// BuildSkillRouterPipeline 由全部启用 skill 插件生成单一路由管线（centag-ops-router）。
//
// 结构（技术方案「单一路由 + skill 分支」模型，对齐 config/initdata agent-skill-router.yaml）：
//   - skill-classifier：router 节点，routing_strategy=llm_classify，
//     routes = {skill 注册名 → <skill>-gen}，default_route=chat-gen；
//     请求 metadata 带 forced_route（显式指定 skill，X-Pipeline-ID 后缀）时跳过 LLM 分类强制路由。
//   - <skill>-gen × N：各 skill 的 generator 分支（system_prompt = manifest skill.system_prompt），
//     挂 RouteConfig{router_node_id: skill-classifier, route_value: <skill>-gen}。
//   - chat-gen：默认 chat 分支（is_default）。
//
// 不含 tool_call_injector（工具范围约束由 edgeag registerTools 实现）。
func BuildSkillRouterPipeline(plugins []skills.SkillPlugin, defaultBackend, defaultModel string) (*pipeline.AgentPatternPipeline, error) {
	var branches []pipeline.PipelineNodeConfig

	routes := make(map[string]interface{})
	for _, p := range plugins {
		if !p.Enabled() {
			continue
		}
		def := p.GetSkillDefinition()
		if def.Name == "" {
			continue
		}
		genID := skillBranchNodeID(def.Name)
		routes[def.Name] = genID
		branches = append(branches, skillBranchGenerator(def, genID, defaultBackend, defaultModel))
	}

	if len(branches) == 0 {
		return nil, fmt.Errorf("build skill router pipeline: no enabled skill")
	}

	classifier := pipeline.PipelineNodeConfig{
		ID:             agentSkillRouterClassifierID,
		Type:           pipeline.NodeTypeRouter,
		Kind:           "routing.decide",
		Implementation: "builtin.router",
		Name:           "Skill Classify",
		Backend:        defaultBackend,
		Model:          defaultModel,
		Config: pipeline.NodeConfig{
			Backend: defaultBackend,
			Model:   defaultModel,
			CustomConfig: map[string]interface{}{
				"routing_strategy": "llm_classify",
				"default_route":    agentSkillRouterChatID,
				"routes":           routes,
			},
			TemplateVars: map[string]string{
				"backend": "system.default_backend",
				"model":   "system.default_model",
			},
		},
		Timeout: 15,
	}

	chatGen := pipeline.PipelineNodeConfig{
		ID:             agentSkillRouterChatID,
		Type:           pipeline.NodeTypeGenerator,
		Kind:           "llm.generate",
		Implementation: "builtin.generator",
		Name:           "General Chat",
		Backend:        defaultBackend,
		Model:          defaultModel,
		Config: pipeline.NodeConfig{
			Backend:        defaultBackend,
			Model:          defaultModel,
			PromptTemplate: "{{input}}",
			SystemPrompt:   "你是一个可靠助手，请直接、准确地回答用户的问题。",
			TemplateVars: map[string]string{
				"backend": "system.default_backend",
				"model":   "system.default_model",
			},
		},
		Timeout: 120,
		DependsOn: []string{
			agentSkillRouterClassifierID,
		},
		RouteConfig: &pipeline.RouteConfig{
			RouterNodeID: agentSkillRouterClassifierID,
			RouteValue:   agentSkillRouterChatID,
			IsDefault:    true,
		},
	}

	nodes := make([]pipeline.PipelineNodeConfig, 0, len(branches)+2)
	nodes = append(nodes, classifier)
	nodes = append(nodes, branches...)
	nodes = append(nodes, chatGen)

	return &pipeline.AgentPatternPipeline{
		SchemaVersion: skillPipelineSchemaVersion,
		ID:            centagOpsRouterPipelineID,
		Name:          "Agent Skill Router",
		Description:   "按用户问题自动选择并执行内置/自定义 skill（LLM 意图分类）",
		Version:       "1.0",
		Nodes:         nodes,
		GlobalConfig: pipeline.GlobalPipelineConfig{
			Timeout:       180,
			MaxRetries:    3,
			BypassOnError: true,
			ParallelLimit: 4,
			LogLevel:      "info",
		},
		Metadata: map[string]interface{}{
			skillPipelineMetadataKey: true,
			"skill_router":           true,
		},
	}, nil
}

// skillBranchGenerator 构建单个 skill 分支 generator 节点。
func skillBranchGenerator(def skills.SkillDefinition, genID, defaultBackend, defaultModel string) pipeline.PipelineNodeConfig {
	return pipeline.PipelineNodeConfig{
		ID:             genID,
		Type:           pipeline.NodeTypeGenerator,
		Kind:           "llm.generate",
		Implementation: "builtin.generator",
		Name:           def.Name,
		Backend:        defaultBackend,
		Model:          defaultModel,
		Config: pipeline.NodeConfig{
			Backend:        defaultBackend,
			Model:          defaultModel,
			PromptTemplate: "{{input}}",
			SystemPrompt:   def.SystemPrompt,
			TemplateVars: map[string]string{
				"backend": "system.default_backend",
				"model":   "system.default_model",
			},
		},
		Timeout: 120,
		DependsOn: []string{
			agentSkillRouterClassifierID,
		},
		RouteConfig: &pipeline.RouteConfig{
			RouterNodeID: agentSkillRouterClassifierID,
			RouteValue:   genID,
		},
	}
}

// registerSkillRouter 为 skill 插件注册表生成并注册单一路由管线（agent-skill-router）。
// 重复注册（CRUD 重建）时幂等覆盖。返回注册的 pipeline id；无启用 skill 时返回空串。
func registerSkillRouter(registry *skills.SkillPluginRegistry, pr *pipeline.PipelineRegistry, defaultBackend, defaultModel string) string {
	return registerSkillRouterWithAdmission(registry, pr, defaultBackend, defaultModel, nil)
}

// registerSkillRouterWithAdmission 在 registerSkillRouter 基础上对每个 manifest 做准入校验。
// admission 为 nil 时跳过准入检查（仅注册）。准入未通过的 skill 不进入 router 分支。
func registerSkillRouterWithAdmission(registry *skills.SkillPluginRegistry, pr *pipeline.PipelineRegistry, defaultBackend, defaultModel string, admission *pipeline.AdmissionChecker) string {
	if registry == nil || pr == nil {
		return ""
	}
	var allowed []skills.SkillPlugin
	for _, p := range registry.ListAll() {
		if !p.Enabled() {
			continue
		}
		if admission != nil && admission.IsEnabled() && admission.CheckPermissions(pipeline.NodePluginDescriptor{
			Implementation: p.Descriptor().Implementation,
			Kind:           p.Descriptor().Kind,
			Permissions:    p.Descriptor().Permissions,
		}).Score < 100 {
			logger.Warnf("agent: skill %s 准入校验未通过，不纳入路由", p.GetSkillDefinition().Name)
			continue
		}
		allowed = append(allowed, p)
	}
	pp, err := BuildSkillRouterPipeline(allowed, defaultBackend, defaultModel)
	if err != nil {
		logger.Warnf("agent: 构建 skill router pipeline 失败: %v", err)
		return ""
	}
	if err := pr.Register(pp); err != nil {
		logger.Warnf("agent: 注册 skill router pipeline %s 失败: %v", pp.ID, err)
		return ""
	}
	logger.Infof("agent: skill router pipeline 已注册，%d 个 skill 分支", len(allowed))
	return pp.ID
}
