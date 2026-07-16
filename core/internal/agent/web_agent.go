package agent

import (
	"fmt"
	"time"
)

// ============================================================================
// Web Agent 接口定义 (Task 2.4)
// ============================================================================

// WebAgent Web（浏览器自动化）Agent 接口。
// WebAgent 在 AgentTemplate 基础上扩展了浏览器交互能力，
// 适用于需要在 Web 页面中执行自动化操作的场景。
type WebAgent interface {
	AgentTemplate

	// OpenBrowser 打开浏览器并导航到指定 URL
	OpenBrowser(url string) error

	// TakeScreenshot 截取当前页面截图，返回 PNG 字节数据
	TakeScreenshot() ([]byte, error)

	// ExecuteJavaScript 在当前页面执行 JavaScript 脚本，返回执行结果
	ExecuteJavaScript(script string) (string, error)

	// WaitForElement 等待指定 CSS 选择器对应的元素出现
	WaitForElement(selector string, timeoutMs int) error

	// ClickElement 点击指定 CSS 选择器对应的元素
	ClickElement(selector string) error

	// FillFormField 向指定 CSS 选择器的表单字段填充值
	FillFormField(selector, value string) error

	// GetPageContent 获取当前页面的完整文本内容
	GetPageContent() (string, error)
}

// ============================================================================
// Web Agent 基础实现 (Task 2.5)
// ============================================================================

// WebAgentTemplate Web Agent 基础模板，提供所有 WebAgent 方法的默认实现。
// 默认实现为 stub 模式，返回友好的错误提示（需要浏览器引擎支持）。
// 具体 Web Agent 可嵌入此结构体并集成真实浏览器引擎（如 Playwright、Puppeteer）。
type WebAgentTemplate struct {
	agentType   AgentType
	displayName string
	description string
	config      *BrowserConfig
}

// NewWebAgentTemplate 创建 Web Agent 基础模板
func NewWebAgentTemplate(agentType AgentType, displayName, description string, config *BrowserConfig) *WebAgentTemplate {
	if config == nil {
		config = DefaultBrowserConfig()
	}
	return &WebAgentTemplate{
		agentType:   agentType,
		displayName: displayName,
		description: description,
		config:      config,
	}
}

// BrowserConfig 返回浏览器配置
func (w *WebAgentTemplate) BrowserConfig() *BrowserConfig { return w.config }

// AgentType 返回 Agent 类型标识
func (w *WebAgentTemplate) AgentType() AgentType { return w.agentType }

// DisplayName 返回展示名称
func (w *WebAgentTemplate) DisplayName() string { return w.displayName }

// Description 返回描述信息
func (w *WebAgentTemplate) Description() string { return w.description }

// ConfigFiles Web Agent 默认无 CLI 配置文件
func (w *WebAgentTemplate) ConfigFiles(info *BackendInfo) ([]ConfigFile, error) {
	return nil, nil
}

// SetupCommand Web Agent 默认无一键安装命令
func (w *WebAgentTemplate) SetupCommand(info *BackendInfo) string { return "" }

// PlatformCommands Web Agent 默认无多平台命令
func (w *WebAgentTemplate) PlatformCommands(info *BackendInfo) PlatformCommands {
	return PlatformCommands{}
}

// VerifyCommand Web Agent 默认无验证命令
func (w *WebAgentTemplate) VerifyCommand(info *BackendInfo) string { return "" }

// Steps Web Agent 默认无配置步骤
func (w *WebAgentTemplate) Steps(info *BackendInfo) []ConfigStep { return nil }

// WriteConfig Web Agent 默认无本地写入
func (w *WebAgentTemplate) WriteConfig(info *BackendInfo) error { return nil }

// --- WebAgent 接口的默认实现（stub 模式） ---

// OpenBrowser 默认实现：stub
func (w *WebAgentTemplate) OpenBrowser(url string) error {
	return fmt.Errorf("WebAgent.OpenBrowser not implemented: browser engine required. URL=%s", url)
}

// TakeScreenshot 默认实现：stub
func (w *WebAgentTemplate) TakeScreenshot() ([]byte, error) {
	return nil, fmt.Errorf("WebAgent.TakeScreenshot not implemented: browser engine required")
}

// ExecuteJavaScript 默认实现：stub
func (w *WebAgentTemplate) ExecuteJavaScript(script string) (string, error) {
	return "", fmt.Errorf("WebAgent.ExecuteJavaScript not implemented: browser engine required")
}

// WaitForElement 默认实现：stub
func (w *WebAgentTemplate) WaitForElement(selector string, timeoutMs int) error {
	return fmt.Errorf("WebAgent.WaitForElement not implemented: browser engine required. selector=%s timeout=%dms", selector, timeoutMs)
}

// ClickElement 默认实现：stub
func (w *WebAgentTemplate) ClickElement(selector string) error {
	return fmt.Errorf("WebAgent.ClickElement not implemented: browser engine required. selector=%s", selector)
}

// FillFormField 默认实现：stub
func (w *WebAgentTemplate) FillFormField(selector, value string) error {
	return fmt.Errorf("WebAgent.FillFormField not implemented: browser engine required. selector=%s", selector)
}

// GetPageContent 默认实现：stub
func (w *WebAgentTemplate) GetPageContent() (string, error) {
	return "", fmt.Errorf("WebAgent.GetPageContent not implemented: browser engine required")
}

// ============================================================================
// 编程 & 教育 Web Agent (Task 2.6)
// ============================================================================

// CodingWebAgent 编程场景 Web Agent。
// 增强代码预览相关的浏览器交互能力。
type CodingWebAgent struct {
	*WebAgentTemplate
}

// NewCodingWebAgent 创建编程 Web Agent
func NewCodingWebAgent(config *BrowserConfig) *CodingWebAgent {
	return &CodingWebAgent{
		WebAgentTemplate: NewWebAgentTemplate(
			AgentCodingWeb,
			"Coding Web Agent",
			"Programming-focused web automation agent for code review, documentation browsing, and online IDE interaction",
			config,
		),
	}
}

// ExecuteJavaScript 编程场景：增强错误处理和代码注入日志
func (c *CodingWebAgent) ExecuteJavaScript(script string) (string, error) {
	wrapped := fmt.Sprintf(`(function() {
  try {
    const __coding_result__ = (function() { %s })();
    if (__coding_result__ !== undefined) {
      return JSON.stringify({ ok: true, result: __coding_result__ });
    }
    return JSON.stringify({ ok: true });
  } catch (e) {
    return JSON.stringify({ ok: false, error: e.message, stack: e.stack });
  }
})()`, script)
	return c.WebAgentTemplate.ExecuteJavaScript(wrapped)
}

// WaitForElement 编程场景：增加超时日志
func (c *CodingWebAgent) WaitForElement(selector string, timeoutMs int) error {
	if timeoutMs <= 0 {
		timeoutMs = c.config.Timeout * 1000
	}
	return c.WebAgentTemplate.WaitForElement(selector, timeoutMs)
}

// EducationWebAgent 教育场景 Web Agent。
// 增强在线学习平台交互能力。
type EducationWebAgent struct {
	*WebAgentTemplate
}

// NewEducationWebAgent 创建教育 Web Agent
func NewEducationWebAgent(config *BrowserConfig) *EducationWebAgent {
	return &EducationWebAgent{
		WebAgentTemplate: NewWebAgentTemplate(
			AgentEducationWeb,
			"Education Web Agent",
			"Education-focused web automation agent for online learning platforms, quiz taking, and course navigation",
			config,
		),
	}
}

// OpenBrowser 教育场景：添加延时以适配学习平台加载
func (e *EducationWebAgent) OpenBrowser(url string) error {
	if err := e.WebAgentTemplate.OpenBrowser(url); err != nil {
		return err
	}
	// 在线学习平台通常需要更多加载时间
	time.Sleep(500 * time.Millisecond)
	return nil
}

// GetPageContent 教育场景：提取结构化学习内容
func (e *EducationWebAgent) GetPageContent() (string, error) {
	content, err := e.WebAgentTemplate.GetPageContent()
	if err != nil {
		return "", err
	}
	// 尝试提取课程结构信息
	script := `(function() {
	  var sections = document.querySelectorAll('h1,h2,h3,.lesson-title,.module-title');
	  if (sections.length === 0) return '';
	  var titles = [];
	  sections.forEach(function(el) { titles.push(el.textContent.trim()); });
	  return JSON.stringify(titles);
	})()`
	e.WebAgentTemplate.ExecuteJavaScript(script) // 静默忽略结构提取失败

	return content, nil
}

// Assert interface compliance
var (
	_ WebAgent = (*WebAgentTemplate)(nil)
	_ WebAgent = (*CodingWebAgent)(nil)
	_ WebAgent = (*EducationWebAgent)(nil)
)

// ============================================================================
// Web Agent 配置模板桩（用于 TemplateRegistry 注册）
// ============================================================================

// webConfigTemplate Web Agent 的 AgentTemplate 桩实现。
// Web Agent 不生成 CLI 配置文件，仅作为类型占位供注册表识别。
type webConfigTemplate struct {
	agentType   AgentType
	displayName string
	description string
}

func newWebConfigTemplate(agentType AgentType, displayName, description string) *webConfigTemplate {
	return &webConfigTemplate{
		agentType:   agentType,
		displayName: displayName,
		description: description,
	}
}

func (w *webConfigTemplate) AgentType() AgentType                          { return w.agentType }
func (w *webConfigTemplate) DisplayName() string                           { return w.displayName }
func (w *webConfigTemplate) Description() string                           { return w.description }
func (w *webConfigTemplate) ConfigFiles(*BackendInfo) ([]ConfigFile, error) { return nil, nil }
func (w *webConfigTemplate) SetupCommand(*BackendInfo) string              { return "" }
func (w *webConfigTemplate) PlatformCommands(*BackendInfo) PlatformCommands { return PlatformCommands{} }
func (w *webConfigTemplate) VerifyCommand(*BackendInfo) string             { return "" }
func (w *webConfigTemplate) Steps(*BackendInfo) []ConfigStep               { return nil }
func (w *webConfigTemplate) WriteConfig(*BackendInfo) error                { return nil }

var (
	_ AgentTemplate = (*webConfigTemplate)(nil)
)
