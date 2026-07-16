package pipeline

import (
	"fmt"
	"strconv"
	"strings"
)

// BusinessPlugin 业务插件接口（扩展 NodePlugin）
// 业务插件是面向特定业务场景（优化、审核、摘要、翻译等）的流水线节点，
// 在 NodePlugin 基础上增加业务类型、依赖关系和输出 schema 等元数据。
type BusinessPlugin interface {
	NodePlugin

	// GetBusinessType 返回业务类型标识（optimize, review, summarize, translate 等）
	GetBusinessType() string

	// GetDependencies 返回插件依赖的基础能力列表（llm.call, storage.read 等）
	GetDependencies() []string

	// GetBusinessMetadata 返回业务插件元数据
	GetBusinessMetadata() BusinessPluginMetadata
}

// BusinessPluginMetadata 业务插件元数据结构
type BusinessPluginMetadata struct {
	BusinessType   string     `json:"business_type"`    // optimize, review, summarize, translate
	Category       string     `json:"category"`         // content, quality, format
	InputFormat    string     `json:"input_format"`     // text, markdown, json
	OutputFormat   string     `json:"output_format"`    // text, markdown, json
	RequiresLLM    bool       `json:"requires_llm"`     // 是否需要调用 LLM
	LLMModel       string     `json:"llm_model"`        // 推荐使用的模型
	PromptTemplate string     `json:"prompt_template"`  // 默认提示词模板
	ConfigSchema   JSONSchema `json:"config_schema"`    // 配置 schema（JSON Schema 格式）
}

// BusinessPluginRegistry 业务插件注册表
// 在 NodeRegistry 基础上提供业务类型维度的索引与发现。
type BusinessPluginRegistry struct {
	pluginsByType map[string][]BusinessPlugin // business_type -> plugins
	all           map[string]BusinessPlugin   // implementation -> plugin
}

// NewBusinessPluginRegistry 创建业务插件注册表
func NewBusinessPluginRegistry() *BusinessPluginRegistry {
	return &BusinessPluginRegistry{
		pluginsByType: make(map[string][]BusinessPlugin),
		all:           make(map[string]BusinessPlugin),
	}
}

// Register 注册业务插件
// 同时将插件添加到全局 NodeRegistry（通过 registryFn 回调）。
func (r *BusinessPluginRegistry) Register(plugin BusinessPlugin, registryFn func(NodePlugin) error) error {
	if plugin == nil {
		return nil
	}
	impl := NormalizeImplementation(plugin.Descriptor().Implementation)
	if impl == "" {
		return nil
	}
	r.all[impl] = plugin
	bt := plugin.GetBusinessType()
	if bt != "" {
		r.pluginsByType[bt] = append(r.pluginsByType[bt], plugin)
	}
	if registryFn != nil {
		return registryFn(plugin)
	}
	return nil
}

// GetByImplementation 按 implementation 标识查询业务插件
func (r *BusinessPluginRegistry) GetByImplementation(impl string) BusinessPlugin {
	return r.all[NormalizeImplementation(impl)]
}

// GetByBusinessType 按业务类型查询插件列表
func (r *BusinessPluginRegistry) GetByBusinessType(businessType string) []BusinessPlugin {
	return r.pluginsByType[businessType]
}

// GetAllBusinessTypes 返回所有已注册的业务类型
func (r *BusinessPluginRegistry) GetAllBusinessTypes() []string {
	types := make([]string, 0, len(r.pluginsByType))
	for t := range r.pluginsByType {
		types = append(types, t)
	}
	return types
}

// GetAll 返回所有已注册的业务插件
func (r *BusinessPluginRegistry) GetAll() []BusinessPlugin {
	plugins := make([]BusinessPlugin, 0, len(r.all))
	for _, p := range r.all {
		plugins = append(plugins, p)
	}
	return plugins
}

// Count 返回已注册的业务插件数量
func (r *BusinessPluginRegistry) Count() int {
	return len(r.all)
}

// PluginVersionConstraint 插件版本约束
// 支持语义化版本范围表达式，如 ">=1.0.0 <2.0.0"
type PluginVersionConstraint struct {
	Expression string `json:"plugin_version,omitempty" yaml:"plugin_version,omitempty"`
}

// CheckVersion 检查插件版本是否满足约束
// constraint 是版本范围表达式，current 是插件当前版本
// 返回 nil 表示满足约束，否则返回错误描述
func CheckPluginVersion(constraint, current string) error {
	if constraint == "" {
		return nil
	}
	if current == "" {
		return fmt.Errorf("plugin has no version, but constraint %q is specified", constraint)
	}
	// 简单实现：仅支持 ">=X.Y.Z <A.B.C" 格式
	// 完整实现应使用 semver 库（如 Masterminds/semver）
	constraint = strings.TrimSpace(constraint)
	current = strings.TrimSpace(current)

	parts := strings.Fields(constraint)
	if len(parts) == 0 {
		return nil
	}

	for i := 0; i < len(parts); i += 2 {
		if i+1 >= len(parts) {
			break
		}
		op := parts[i]
		ver := parts[i+1]
		if !strings.HasPrefix(ver, "v") {
			ver = "v" + ver
		}
		cur := current
		if !strings.HasPrefix(cur, "v") {
			cur = "v" + cur
		}

		cmp := compareSemVer(cur, ver)
		switch op {
		case ">=":
			if cmp < 0 {
				return fmt.Errorf("plugin version %s does not satisfy constraint %s %s", current, op, ver)
			}
		case "<=":
			if cmp > 0 {
				return fmt.Errorf("plugin version %s does not satisfy constraint %s %s", current, op, ver)
			}
		case ">":
			if cmp <= 0 {
				return fmt.Errorf("plugin version %s does not satisfy constraint %s %s", current, op, ver)
			}
		case "<":
			if cmp >= 0 {
				return fmt.Errorf("plugin version %s does not satisfy constraint %s %s", current, op, ver)
			}
		case "==", "=":
			if cmp != 0 {
				return fmt.Errorf("plugin version %s does not satisfy constraint %s %s", current, op, ver)
			}
		}
	}
	return nil
}

// compareSemVer 比较两个语义化版本号
// 返回 -1 表示 a < b, 0 表示 a == b, 1 表示 a > b
func compareSemVer(a, b string) int {
	trimVer := func(v string) string {
		v = strings.TrimPrefix(v, "v")
		v = strings.TrimPrefix(v, "V")
		return v
	}
	a, b = trimVer(a), trimVer(b)

	split := func(v string) []int {
		parts := strings.Split(v, ".")
		nums := make([]int, 3)
		for i, p := range parts {
			if i >= 3 {
				break
			}
			n, _ := strconv.Atoi(strings.Split(p, "-")[0])
			nums[i] = n
		}
		return nums
	}

	va, vb := split(a), split(b)
	for i := 0; i < 3; i++ {
		if va[i] < vb[i] {
			return -1
		}
		if va[i] > vb[i] {
			return 1
		}
	}
	return 0
}

// MinProxyclawVersionCheck 检查插件所需的最小 Proxyclaw 版本
func MinProxyclawVersionCheck(descriptor NodePluginDescriptor, proxyclawVersion string) error {
	minVer := descriptor.MinProxyclawVersion
	if minVer == "" {
		return nil
	}
	if proxyclawVersion == "" {
		return fmt.Errorf("plugin %q requires centag version >= %s, but current version is unknown",
			descriptor.Implementation, minVer)
	}
	return CheckPluginVersion(">="+minVer, proxyclawVersion)
}

// CleanJSONContent 从 LLM 响应中提取 JSON 内容（导出给外部插件使用）
// 按优先级依次尝试：
//  1. 整体就是一个合法 JSON
//  2. markdown 代码块 ```json ... ```
//  3. 正文中第一个完整 { ... } 对象
func CleanJSONContent(content string) string {
	return cleanJSONContent(content)
}


