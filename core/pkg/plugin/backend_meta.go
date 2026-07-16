package plugin

import (
	"sync"
)

// BackendMeta 定义后端插件的元数据信息。
// 用于管理端发现「当前二进制支持哪些 backend type」，
// 并提供 DefaultBaseURL / KeyHelp / ConfigSchema 供 WebUI 动态表单。
type BackendMeta struct {
	// Type 后端类型标识符（如 "openai"、"anthropic"、"gemini"）。
	// 必须与配置里的 type 字段一致，不是 Name() 返回值。
	Type string `json:"type"`

	// Name 后端显示名称（如 "OpenAI"、"Anthropic"）。
	Name string `json:"name"`

	// DefaultBaseURL 默认的 API 基础 URL。
	DefaultBaseURL string `json:"default_base_url"`

	// KeyHelp API Key 格式说明或提示信息。
	KeyHelp string `json:"key_help"`

	// ConfigSchema JSON Schema 格式的配置 schema，供 WebUI 动态表单渲染。
	// 可为 nil，表示使用默认表单。
	ConfigSchema map[string]any `json:"config_schema,omitempty"`

	// Capabilities 后端支持的能力列表（如 "chat", "embeddings", "streaming"）。
	Capabilities []string `json:"capabilities"`

	// AuthSchemes 支持的认证方案（如 "bearer", "api_key", "oauth"）。
	AuthSchemes []string `json:"auth_schemes"`
}

// 全局 BackendMeta 注册表（并发安全）
var (
	backendMetaRegistry = &sync.Map{}
)

// RegisterBackendMeta 注册后端元数据。
// 应在后端插件的 init() 中调用。
func RegisterBackendMeta(meta BackendMeta) {
	backendMetaRegistry.Store(meta.Type, meta)
}

// ListBackendMetas 返回所有已注册的后端元数据列表。
func ListBackendMetas() []BackendMeta {
	var metas []BackendMeta
	backendMetaRegistry.Range(func(key, value interface{}) bool {
		metas = append(metas, value.(BackendMeta))
		return true
	})
	return metas
}

// GetBackendMeta 根据类型名获取后端元数据。
func GetBackendMeta(typeName string) (BackendMeta, bool) {
	value, ok := backendMetaRegistry.Load(typeName)
	if !ok {
		return BackendMeta{}, false
	}
	return value.(BackendMeta), true
}
