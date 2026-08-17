package skills

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// SkillPluginSchemaVersion 是 skill 插件 manifest 的契约版本。
const SkillPluginSchemaVersion = "centag.agent-skill/v1alpha1"

// SkillPluginKind 标识 skill 插件（区别于节点插件 kind: pipeline.node）。
const SkillPluginKind = "agent.skill"

// CentagOpsRouterPipelineID 全部 skill 共用的单一路由管线 id。
// 由 skill 插件注册时自动生成一次（centag-ops-router），router 节点按用户问题
// 自动分类路由到对应 skill 分支，替代「每个 skill 一条独立 pipeline」的挂接模型。
const CentagOpsRouterPipelineID = "centag-ops-router"

// ForcedRoutePipelineID 返回显式指定 skill 时使用的 X-Pipeline-ID 值。
// 约定：centag-ops-router:<skill>，proxy 侧解析出强制路由并跳过 LLM 分类。
func ForcedRoutePipelineID(skillName string) string {
	return CentagOpsRouterPipelineID + ":" + skillName
}

// SkillPlugin 接口：agent skill 插件契约。
// skill 是独立实体，pipeline 是执行载体，通过 agent-skill-router 共享路由管线挂接。
type SkillPlugin interface {
	Descriptor() SkillPluginDescriptor
	GetSkillDefinition() SkillDefinition
	Enabled() bool
	Internal() bool
	PipelineID() string
}

// SkillPluginDescriptor skill 插件描述，安全字段对齐 pipeline.NodePluginDescriptor。
type SkillPluginDescriptor struct {
	APIVersion   string   `json:"api_version"`
	Implementation string `json:"implementation"`
	Name         string   `json:"name"`
	Kind         string   `json:"kind"`
	Version      string   `json:"version"`
	Description  string   `json:"description,omitempty"`
	Permissions  []string `json:"permissions,omitempty"`
	Signature    string   `json:"signature,omitempty"`    // Ed25519（复用 PluginSecurityValidator）
	ExpectedHash string   `json:"expected_hash,omitempty"` // SHA-256（复用 ManifestHashConfig 机制）
}

// SkillDefinition skill 业务定义。
type SkillDefinition struct {
	Name         string   `json:"name"`      // skill 注册名（映射 key）
	Category     string   `json:"category"`
	Tools        []string `json:"tools"`
	Steps        []string `json:"steps"`
	SystemPrompt string   `json:"system_prompt"`
}

// skillPluginManifest manifest 的 YAML 反序列化载体。
// skill.system_prompt 使用 string 而非 | 块扫描标量，yaml.v3 对块标量同样解码为 string。
type skillPluginManifest struct {
	APIVersion     string        `yaml:"api_version"`
	Implementation string        `yaml:"implementation"`
	Name           string        `yaml:"name"`
	Kind           string        `yaml:"kind"`
	Version        string        `yaml:"version"`
	Description    string        `yaml:"description"`
	Permissions    []string      `yaml:"permissions"`
	Signature      string        `yaml:"signature"`
	ExpectedHash   string        `yaml:"expected_hash"`
	Skill          skillManifest `yaml:"skill"`
}

type skillManifest struct {
	Name         string   `yaml:"name"`
	Category     string   `yaml:"category"`
	Enabled      *bool    `yaml:"enabled"`
	Internal     *bool    `yaml:"internal"`
	Tools        []string `yaml:"tools"`
	Steps        []string `yaml:"steps"`
	SystemPrompt string   `yaml:"system_prompt"`
}

// skillPlugin manifest 的具体实现。
type skillPlugin struct {
	descriptor SkillPluginDescriptor
	definition SkillDefinition
	enabled    bool
	internal   bool
}

func (p *skillPlugin) Descriptor() SkillPluginDescriptor    { return p.descriptor }
func (p *skillPlugin) GetSkillDefinition() SkillDefinition { return p.definition }
func (p *skillPlugin) Enabled() bool                       { return p.enabled }
func (p *skillPlugin) Internal() bool                      { return p.internal }
func (p *skillPlugin) PipelineID() string                  { return CentagOpsRouterPipelineID }

// ParseSkillPluginManifest 将 agent-skill-*.yaml 内容解析为 SkillPlugin。
// 解析规则：
//   - 缺 api_version 或 kind 不匹配 → 拒绝
//   - 缺少 implementation / skill.name → 拒绝
//   - 未知字段忽略（yaml.v3 默认行为）
//   - enabled / internal 缺省为 true
func ParseSkillPluginManifest(data []byte) (SkillPlugin, error) {
	var m skillPluginManifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse skill manifest: %w", err)
	}

	if m.APIVersion == "" {
		return nil, fmt.Errorf("parse skill manifest: missing api_version")
	}
	if m.APIVersion != SkillPluginSchemaVersion {
		return nil, fmt.Errorf("parse skill manifest: unsupported api_version %q, want %q", m.APIVersion, SkillPluginSchemaVersion)
	}
	if m.Kind != SkillPluginKind {
		return nil, fmt.Errorf("parse skill manifest: kind %q, want %q", m.Kind, SkillPluginKind)
	}
	if m.Implementation == "" {
		return nil, fmt.Errorf("parse skill manifest: missing implementation")
	}
	if m.Skill.Name == "" {
		return nil, fmt.Errorf("parse skill manifest: missing skill.name")
	}

	enabled := true
	if m.Skill.Enabled != nil {
		enabled = *m.Skill.Enabled
	}
	internal := true
	if m.Skill.Internal != nil {
		internal = *m.Skill.Internal
	}

	descriptor := SkillPluginDescriptor{
		APIVersion:     m.APIVersion,
		Implementation: m.Implementation,
		Name:           m.Name,
		Kind:           m.Kind,
		Version:        m.Version,
		Description:    m.Description,
		Permissions:    m.Permissions,
		Signature:      m.Signature,
		ExpectedHash:   m.ExpectedHash,
	}

	definition := SkillDefinition{
		Name:         m.Skill.Name,
		Category:     m.Skill.Category,
		Tools:        m.Skill.Tools,
		Steps:        m.Skill.Steps,
		SystemPrompt: m.Skill.SystemPrompt,
	}

	return &skillPlugin{
		descriptor: descriptor,
		definition: definition,
		enabled:    enabled,
		internal:   internal,
	}, nil
}

// MarshalSkillPluginManifest 将 SkillPlugin 序列化为 manifest YAML。
// 供自定义 skill 表单生成与持久化使用。
func MarshalSkillPluginManifest(p SkillPlugin) ([]byte, error) {
	enabled := p.Enabled()
	internal := p.Internal()
	desc := p.Descriptor()
	def := p.GetSkillDefinition()
	m := skillPluginManifest{
		APIVersion:     desc.APIVersion,
		Implementation: desc.Implementation,
		Name:           desc.Name,
		Kind:           desc.Kind,
		Version:        desc.Version,
		Description:    desc.Description,
		Permissions:    desc.Permissions,
		Signature:      desc.Signature,
		ExpectedHash:   desc.ExpectedHash,
		Skill: skillManifest{
			Name:         def.Name,
			Category:     def.Category,
			Enabled:      &enabled,
			Internal:     &internal,
			Tools:        def.Tools,
			Steps:        def.Steps,
			SystemPrompt: def.SystemPrompt,
		},
	}
	return yaml.Marshal(&m)
}

// PipelineIDFromImplementation 由 implementation 推导 pipeline id（去 builtin./custom. 前缀）。
// builtin.agent-skill-status-check → agent-skill-status-check
func PipelineIDFromImplementation(implementation string) string {
	for _, prefix := range []string{"builtin.", "custom."} {
		if len(implementation) > len(prefix) && implementation[:len(prefix)] == prefix {
			return implementation[len(prefix):]
		}
	}
	return implementation
}

// SkillFromPlugin 将 SkillPlugin 渲染为可执行的 Skill（manifest 为权威来源）。
// Prompt 取 manifest 的 system_prompt；Name 用 manifest 的 skill.name。
func SkillFromPlugin(p SkillPlugin) *Skill {
	def := p.GetSkillDefinition()
	s := &Skill{
		Name:        def.Name,
		Description: p.Descriptor().Description,
		Version:     p.Descriptor().Version,
		Category:    def.Category,
		Tools:       def.Tools,
		Steps:       def.Steps,
		Enabled:     p.Enabled(),
		Internal:    p.Internal(),
		Prompt:      def.SystemPrompt,
	}
	if s.Description == "" {
		s.Description = def.Name
	}
	if s.Version == "" {
		s.Version = "1.0.0"
	}
	return s
}
