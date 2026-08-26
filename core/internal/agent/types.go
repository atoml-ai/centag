package agent

// AgentType 支持的 Agent 工具类型
type AgentType string

const (
	AgentClaudeCode    AgentType = "claude-code"
	AgentClaudeDesktop AgentType = "claude-desktop"
	AgentCodex         AgentType = "codex"
	AgentGeminiCLI     AgentType = "gemini-cli"
	AgentGrokBuild     AgentType = "grok-build"
	AgentOpenCode      AgentType = "opencode"
	AgentOpenClaw      AgentType = "openclaw"
	AgentPi            AgentType = "pi"
	AgentHermes        AgentType = "hermes"
	AgentCodeBuddy     AgentType = "codebuddy"
	AgentWorkBuddy     AgentType = "workbuddy"
	AgentTrae          AgentType = "trae"

	// TUI Agent 类型（终端用户界面）
	AgentCodingTUI    AgentType = "coding-tui"
	AgentEducationTUI AgentType = "education-tui"

	// Web Agent 类型（浏览器自动化）
	AgentCodingWeb    AgentType = "coding-web"
	AgentEducationWeb AgentType = "education-web"
)

// AgentCategory Agent 交互界面类别
type AgentCategory string

const (
	AgentCategoryCLI     AgentCategory = "cli"
	AgentCategoryTUI     AgentCategory = "tui"
	AgentCategoryWeb     AgentCategory = "web"
	AgentCategoryDesktop AgentCategory = "desktop"
)

// NotificationLevel 通知级别
type NotificationLevel int

const (
	NotificationInfo    NotificationLevel = iota
	NotificationSuccess
	NotificationWarning
	NotificationError
)

// String 返回通知级别的字符串表示
func (l NotificationLevel) String() string {
	switch l {
	case NotificationInfo:
		return "info"
	case NotificationSuccess:
		return "success"
	case NotificationWarning:
		return "warning"
	case NotificationError:
		return "error"
	default:
		return "unknown"
	}
}

// ConfigFile 需写入的配置文件
type ConfigFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Append  bool   `json:"append,omitempty"`
}

// ConfigStep 手动配置步骤
type ConfigStep struct {
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Code        string `json:"code,omitempty"`
}

// GenerateConfigRequest 生成配置的请求
type GenerateConfigRequest struct {
	BackendID  string `json:"backend_id,omitempty"`
	AgentType  string `json:"agent_type" binding:"required"`
	Model      string `json:"model,omitempty"`
	PipelineID string `json:"pipeline_id,omitempty"`
	Host       string `json:"host,omitempty"`
	Port       int    `json:"port,omitempty"`
	// ViaProxy 指定后端经代理接入（backend_id+via_proxy）：
	// Agent 配置仍写 Centag 地址与代理密钥，模型用真实名；
	// 运行期裸模型名命中透明流水线钉死该后端，写入时同步系统默认出站。
	ViaProxy bool `json:"via_proxy,omitempty"`
}

// PlatformCommands 按操作系统分类的写入命令
type PlatformCommands struct {
	MacOS   string `json:"macos,omitempty"`
	Linux   string `json:"linux,omitempty"`
	Windows string `json:"windows,omitempty"`
}

// WriteConfigRequest 写入配置请求（桌面版调用）
type WriteConfigRequest struct {
	BackendID  string `json:"backend_id,omitempty"`
	AgentType  string `json:"agent_type" binding:"required"`
	Model      string `json:"model,omitempty"`
	PipelineID string `json:"pipeline_id,omitempty"`
	Host       string `json:"host,omitempty"`
	Port       int    `json:"port,omitempty"`
	// ViaProxy 同 GenerateConfigRequest；写入成功后同步系统默认后端/模型。
	ViaProxy bool `json:"via_proxy,omitempty"`
}

// WriteConfigResponse 写入结果
type WriteConfigResponse struct {
	AgentType string       `json:"agent_type"`
	Success   bool         `json:"success"`
	Written   []ConfigFile `json:"written,omitempty"`
	Message   string       `json:"message,omitempty"`
	// RestartRequired 写入后需重启对应客户端才能生效（桌面 IDE 等）。
	RestartRequired bool `json:"restart_required,omitempty"`
	// GuideExported 表示仅导出了 UI 接入说明，并未改写代理相关配置。
	GuideExported bool `json:"guide_exported,omitempty"`
}

// RestoreConfigRequest 恢复 Agent 本地默认配置
type RestoreConfigRequest struct {
	AgentType string `json:"agent_type" binding:"required"`
}

// RestoreConfigResponse 恢复结果
type RestoreConfigResponse struct {
	AgentType string          `json:"agent_type"`
	Success   bool            `json:"success"`
	Results   []RestoreResult `json:"results,omitempty"`
	Message   string          `json:"message,omitempty"`
}

// GenerateConfigResponse 生成的配置响应
type GenerateConfigResponse struct {
	AgentType   string           `json:"agent_type"`
	BackendName string           `json:"backend_name"`
	Description string           `json:"description"`
	Commands    PlatformCommands `json:"commands"`
	Files       []ConfigFile     `json:"files,omitempty"`
	Steps       []ConfigStep     `json:"steps,omitempty"`
	VerifyCmd   string           `json:"verify_cmd,omitempty"`
}

// BackendInfo 后端信息摘要（模板渲染所需字段）
type BackendInfo struct {
	ID      string
	Name    string
	BaseURL string
	APIKey  string
	Type    string
	Model   string
	Host    string
	Port    int
}

// WriteMode 本地配置写入模式
const (
	WriteModeOverwrite = "overwrite" // 覆盖型（切换当前供应商）
	WriteModeMerge     = "merge"     // 累加共存（合并进现有配置）
	WriteModeNone      = "none"      // 无本地写配置
)

// AgentSetupMeta Agent 接入元数据（卡片展示 / 调试用）
// 预定义的 Vendor 品牌常量。新增 Agent 时请复用这些常量，或按需追加。
const (
	VendorAnthropic  = "Anthropic"
	VendorOpenAI     = "OpenAI"
	VendorGoogle     = "Google"
	VendorXAI       = "xAI"
	VendorTencent   = "腾讯云"
	VendorByteDance = "字节跳动"
	VendorOpenCode  = "OpenCode"
	VendorOpenClaw  = "OpenClaw"
	VendorPi        = "Pi"
	VendorHermes    = "Hermes"
)

type AgentSetupMeta struct {
	Category     AgentCategory `json:"category"`
	Vendor       string        `json:"vendor"` // 品牌分组，使用本文件顶部的 VendorXxx 常量
	WriteMode    string        `json:"write_mode"` // overwrite | merge | none
	ConfigPaths  []string      `json:"config_paths"`
	KeyFields    []string      `json:"key_fields"`
	ConfigMethod string        `json:"config_method"` // 给人看的写入说明
	InstallURL   string        `json:"install_url"`
	InstallHint  string        `json:"install_hint"`
	// AccessMethods 显式接入能力（write_config / ui_guide / wrap_cli / builtin）。
	AccessMethods []AccessMethod `json:"access_methods,omitempty"`
	CompanionCLI  *CompanionCLI  `json:"companion_cli,omitempty"`
	UIGuide       *UIGuide       `json:"ui_guide,omitempty"`
	// VerifiedWrite 表示「写入配置」接入方式已通过维护者本地验证。
	VerifiedWrite bool `json:"verified_write,omitempty"`
	// VerifiedWrap 表示「wrap / 系统代理」接入方式已通过维护者本地验证。
	VerifiedWrap bool `json:"verified_wrap,omitempty"`
	// VerifiedUI 表示「UI 指引」接入方式已通过维护者本地验证。
	VerifiedUI bool `json:"verified_ui,omitempty"`
}

// AgentTemplate 各 Agent 工具的配置模板接口
type AgentTemplate interface {
	AgentType() AgentType
	DisplayName() string
	Description() string
	Meta() AgentSetupMeta
	ConfigFiles(info *BackendInfo) ([]ConfigFile, error)
	SetupCommand(info *BackendInfo) string
	PlatformCommands(info *BackendInfo) PlatformCommands
	VerifyCommand(info *BackendInfo) string
	Steps(info *BackendInfo) []ConfigStep
	WriteConfig(info *BackendInfo) error
}

// TemplateRegistry 模板注册表
type TemplateRegistry struct {
	templates map[AgentType]AgentTemplate
}

// NewTemplateRegistry 创建注册表
func NewTemplateRegistry() *TemplateRegistry {
	r := &TemplateRegistry{
		templates: make(map[AgentType]AgentTemplate),
	}
	r.registerDefaults()
	return r
}

func (r *TemplateRegistry) registerDefaults() {
	// CLI Agents
	r.Register(&ClaudeCodeTemplate{})
	r.Register(&ClaudeDesktopTemplate{})
	r.Register(&CodexTemplate{})
	r.Register(&GeminiTemplate{})
	r.Register(&GrokBuildTemplate{})
	r.Register(&OpenCodeTemplate{})
	r.Register(&OpenClawTemplate{})
	r.Register(&PiTemplate{})
	r.Register(&HermesTemplate{})
	r.Register(&CodeBuddyTemplate{})
	r.Register(&WorkBuddyTemplate{})
	r.Register(&TraeTemplate{})

	// TUI Agents
	r.Register(newTUIConfigTemplate(AgentCodingTUI, "Coding TUI Agent", "编程场景终端交互 Agent"))
	r.Register(newTUIConfigTemplate(AgentEducationTUI, "Education TUI Agent", "教育场景终端交互 Agent"))

	// Web Agents
	r.Register(newWebConfigTemplate(AgentCodingWeb, "Coding Web Agent", "编程场景 Web 自动化 Agent"))
	r.Register(newWebConfigTemplate(AgentEducationWeb, "Education Web Agent", "教育场景 Web 自动化 Agent"))
}

// Register 注册模板
func (r *TemplateRegistry) Register(t AgentTemplate) {
	r.templates[t.AgentType()] = t
}

// Get 获取模板
func (r *TemplateRegistry) Get(at AgentType) (AgentTemplate, bool) {
	t, ok := r.templates[at]
	return t, ok
}

// List 列出所有 Agent 类型
func (r *TemplateRegistry) List() []AgentType {
	var types []AgentType
	for at := range r.templates {
		types = append(types, at)
	}
	return types
}

// ============================================================================
// TUI Agent 支持类型
// ============================================================================

// StatusBarInfo 状态栏渲染所需的上下文信息
type StatusBarInfo struct {
	Mode     string // 当前模式（NORMAL / INSERT / COMMAND）
	FilePath string // 当前文件路径
	Modified bool   // 是否有未保存修改
	Position int    // 光标行号
	Encoding string
	Language string
}

// ProgressInfo 进度条渲染信息
type ProgressInfo struct {
	Current    int
	Total      int
	Message    string
	Percentage float64
}

// CodeHighlightOptions 代码高亮配置
type CodeHighlightOptions struct {
	Language        string
	ShowLineNumbers bool
	Theme           string // dark / light
}

// DiffViewOptions Diff 视图配置
type DiffViewOptions struct {
	ShowLineNumbers bool
	ContextLines    int
	SideBySide      bool // true=并排显示, false=统一视图
}

// UserChoice 交互式用户选项
type UserChoice struct {
	Key         string
	Label       string
	Description string
	Selected    bool // 默认选中
}

// Notification 通知信息
type Notification struct {
	Level    NotificationLevel
	Message  string
	Duration int // ms, 0 表示粘性通知（需手动关闭）
}

// ============================================================================
// Web Agent 支持类型
// ============================================================================

// BrowserConfig 浏览器引擎配置
type BrowserConfig struct {
	Headless       bool
	ViewportWidth  int
	ViewportHeight int
	Timeout        int    // 页面加载超时（秒）
	UserAgent      string // 自定义 User-Agent
}

// DefaultBrowserConfig 返回默认浏览器配置
func DefaultBrowserConfig() *BrowserConfig {
	return &BrowserConfig{
		Headless:       true,
		ViewportWidth:  1280,
		ViewportHeight: 720,
		Timeout:        30,
	}
}
