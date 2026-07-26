package agent

import (
	"fmt"
	"strings"
)

// ============================================================================
// TUI Agent 接口定义 (Task 2.1)
// ============================================================================

// TUIAgent TUI（终端用户界面）Agent 接口。
// TUIAgent 在 AgentTemplate 基础上扩展了终端渲染和用户交互能力，
// 适用于需要在终端中展示丰富 UI 的场景。
type TUIAgent interface {
	AgentTemplate

	// RenderStatusBar 渲染状态栏（模式、文件信息、光标位置等）
	RenderStatusBar(info *StatusBarInfo) string

	// RenderProgress 渲染进度条
	RenderProgress(info *ProgressInfo) string

	// RenderCodeHighlight 渲染带语法高亮的代码块
	RenderCodeHighlight(code string, opts *CodeHighlightOptions) string

	// RenderDiff 渲染 Diff 对比视图
	RenderDiff(oldCode, newCode string, opts *DiffViewOptions) string

	// PromptUserChoice 显示交互式选择菜单，返回选中项的索引
	PromptUserChoice(choices []UserChoice) (int, error)

	// ShowNotification 显示通知消息
	ShowNotification(notification *Notification) string
}

// ============================================================================
// TUI Agent 基础实现 (Task 2.2)
// ============================================================================

// TUIAgentTemplate TUI Agent 基础模板，提供所有 TUIAgent 方法的默认实现。
// 具体 TUI Agent 可嵌入此结构体并重写需要自定义的方法。
type TUIAgentTemplate struct {
	agentType   AgentType
	displayName string
	description string
}

// NewTUIAgentTemplate 创建 TUI Agent 基础模板
func NewTUIAgentTemplate(agentType AgentType, displayName, description string) *TUIAgentTemplate {
	return &TUIAgentTemplate{
		agentType:   agentType,
		displayName: displayName,
		description: description,
	}
}

// AgentType 返回 Agent 类型标识
func (t *TUIAgentTemplate) AgentType() AgentType { return t.agentType }

// DisplayName 返回展示名称
func (t *TUIAgentTemplate) DisplayName() string { return t.displayName }

// Description 返回描述信息
func (t *TUIAgentTemplate) Description() string { return t.description }

// ConfigFiles TUI Agent 默认无配置文件（CLI 配置领域）
func (t *TUIAgentTemplate) ConfigFiles(info *BackendInfo) ([]ConfigFile, error) {
	return nil, nil
}

// SetupCommand TUI Agent 默认无一键安装命令
func (t *TUIAgentTemplate) SetupCommand(info *BackendInfo) string { return "" }

// PlatformCommands TUI Agent 默认无多平台命令
func (t *TUIAgentTemplate) PlatformCommands(info *BackendInfo) PlatformCommands {
	return PlatformCommands{}
}

// VerifyCommand TUI Agent 默认无验证命令
func (t *TUIAgentTemplate) VerifyCommand(info *BackendInfo) string { return "" }

// Steps TUI Agent 默认无配置步骤
func (t *TUIAgentTemplate) Steps(info *BackendInfo) []ConfigStep { return nil }

// WriteConfig TUI Agent 默认无本地写入
func (t *TUIAgentTemplate) WriteConfig(info *BackendInfo) error { return nil }

// Meta TUI Agent 无本地写配置接入
func (t *TUIAgentTemplate) Meta() AgentSetupMeta {
	return AgentSetupMeta{
		Category:      AgentCategoryTUI,
		WriteMode:     WriteModeNone,
		ConfigPaths:   nil,
		KeyFields:     nil,
		ConfigMethod:  "TUI Agent 不通过写入本地配置文件接入 Centag；由进程内路由/流水线绑定。",
		InstallURL:    "",
		InstallHint:   "内置能力，无需单独安装 CLI",
		AccessMethods: []AccessMethod{AccessBuiltin},
	}
}

// --- TUIAgent 接口的默认实现 ---

// RenderStatusBar 默认状态栏渲染（简洁模式 → 文件:行号）
func (t *TUIAgentTemplate) RenderStatusBar(info *StatusBarInfo) string {
	if info == nil {
		return "-- TUI Agent --"
	}
	modified := " "
	if info.Modified {
		modified = "+"
	}
	return fmt.Sprintf("[%s]%s %s:%d | %s",
		info.Mode, modified, info.FilePath, info.Position, info.Encoding)
}

// RenderProgress 默认进度条渲染（文本进度条 ███░░░ 50%）
func (t *TUIAgentTemplate) RenderProgress(info *ProgressInfo) string {
	if info == nil || info.Total <= 0 {
		return ""
	}
	barWidth := 40
	filled := int(float64(info.Current) / float64(info.Total) * float64(barWidth))
	if filled > barWidth {
		filled = barWidth
	}

	var bar strings.Builder
	for i := 0; i < filled; i++ {
		bar.WriteRune('█')
	}
	for i := filled; i < barWidth; i++ {
		bar.WriteRune('░')
	}

	pct := float64(info.Current) / float64(info.Total) * 100
	msg := info.Message
	if msg == "" {
		msg = fmt.Sprintf("%d/%d", info.Current, info.Total)
	}
	return fmt.Sprintf("%s %.1f%% %s", bar.String(), pct, msg)
}

// RenderCodeHighlight 默认代码渲染（无语法高亮，纯文本 + 可选行号）
func (t *TUIAgentTemplate) RenderCodeHighlight(code string, opts *CodeHighlightOptions) string {
	if opts == nil {
		opts = &CodeHighlightOptions{}
	}
	if !opts.ShowLineNumbers {
		return code
	}

	lines := splitLines(code)
	var result string
	digitCount := len(fmt.Sprintf("%d", len(lines)))
	format := fmt.Sprintf("%%%dd │ %%s\n", digitCount)
	for i, line := range lines {
		result += fmt.Sprintf(format, i+1, line)
	}
	return result
}

// RenderDiff 默认 Diff 视图渲染（简约统一 diff）
func (t *TUIAgentTemplate) RenderDiff(oldCode, newCode string, opts *DiffViewOptions) string {
	if opts == nil {
		opts = &DiffViewOptions{ContextLines: 3}
	}
	oldLines := splitLines(oldCode)
	newLines := splitLines(newCode)
	maxLen := len(oldLines)
	if len(newLines) > maxLen {
		maxLen = len(newLines)
	}

	var result string
	for i := 0; i < maxLen; i++ {
		oldLine, newLine := "", ""
		if i < len(oldLines) {
			oldLine = oldLines[i]
		}
		if i < len(newLines) {
			newLine = newLines[i]
		}
		if oldLine == newLine {
			result += fmt.Sprintf("  %s\n", oldLine)
		} else {
			if oldLine != "" {
				result += fmt.Sprintf("- %s\n", oldLine)
			}
			if newLine != "" {
				result += fmt.Sprintf("+ %s\n", newLine)
			}
		}
	}
	return result
}

// PromptUserChoice 默认交互选择（返回第一项）
func (t *TUIAgentTemplate) PromptUserChoice(choices []UserChoice) (int, error) {
	if len(choices) == 0 {
		return -1, fmt.Errorf("no choices available")
	}
	// 默认行为：返回第一个选项或 Selected=true 的选项
	for i, c := range choices {
		if c.Selected {
			return i, nil
		}
	}
	return 0, nil
}

// ShowNotification 默认通知渲染
func (t *TUIAgentTemplate) ShowNotification(notification *Notification) string {
	if notification == nil {
		return ""
	}
	levelMarkers := map[NotificationLevel]string{
		NotificationInfo:    "ℹ",
		NotificationSuccess: "✓",
		NotificationWarning: "⚠",
		NotificationError:   "✗",
	}
	marker := levelMarkers[notification.Level]
	if marker == "" {
		marker = "•"
	}
	return fmt.Sprintf("%s %s", marker, notification.Message)
}

// --- 内部工具函数 ---

// splitLines 按换行符分割文本
func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

// ============================================================================
// 编程 & 教育 TUI Agent (Task 2.3)
// ============================================================================

// CodingTUIAgent 编程场景 TUI Agent。
// 重写代码高亮和 Diff 渲染，提供更专业的编程工具视图。
type CodingTUIAgent struct {
	*TUIAgentTemplate
}

// NewCodingTUIAgent 创建编程 TUI Agent
func NewCodingTUIAgent() *CodingTUIAgent {
	return &CodingTUIAgent{
		TUIAgentTemplate: NewTUIAgentTemplate(
			AgentCodingTUI,
			"Coding TUI Agent",
			"Programming-focused terminal user interface agent with syntax highlighting and diff view",
		),
	}
}

// RenderCodeHighlight 编程场景：增强代码高亮（语言标识 + 行号）
func (c *CodingTUIAgent) RenderCodeHighlight(code string, opts *CodeHighlightOptions) string {
	if opts == nil {
		opts = &CodeHighlightOptions{
			ShowLineNumbers: true,
			Theme:           "dark",
		}
	}
	if opts.Language == "" {
		opts.Language = "auto"
	}

	result := fmt.Sprintf("```%s\n", opts.Language)
	result += c.TUIAgentTemplate.RenderCodeHighlight(code, opts)
	result += "```"
	return result
}

// RenderDiff 编程场景：并排 Diff 视图
func (c *CodingTUIAgent) RenderDiff(oldCode, newCode string, opts *DiffViewOptions) string {
	if opts == nil {
		opts = &DiffViewOptions{
			ShowLineNumbers: true,
			ContextLines:    3,
			SideBySide:      true,
		}
	}
	if opts.SideBySide {
		return c.renderSideBySideDiff(oldCode, newCode, opts)
	}
	return c.TUIAgentTemplate.RenderDiff(oldCode, newCode, opts)
}

// renderSideBySideDiff 并排 diff 渲染
func (c *CodingTUIAgent) renderSideBySideDiff(oldCode, newCode string, opts *DiffViewOptions) string {
	oldLines := splitLines(oldCode)
	newLines := splitLines(newCode)
	maxLen := len(oldLines)
	if len(newLines) > maxLen {
		maxLen = len(newLines)
	}

	var result string
	for i := 0; i < maxLen; i++ {
		oldStr := ""
		if i < len(oldLines) {
			oldStr = oldLines[i]
		}
		newStr := ""
		if i < len(newLines) {
			newStr = newLines[i]
		}
		if oldStr == newStr {
			result += fmt.Sprintf("  %-50s │   %s\n", oldStr, newStr)
		} else {
			oldPart := ""
			if i < len(oldLines) {
				oldPart = fmt.Sprintf("- %s", oldLines[i])
			}
			newPart := ""
			if i < len(newLines) {
				newPart = fmt.Sprintf("+ %s", newLines[i])
			}
			result += fmt.Sprintf("%-52s │ %s\n", oldPart, newPart)
		}
	}
	return result
}

// RenderStatusBar 编程状态栏：显示当前模式和编程语言
func (c *CodingTUIAgent) RenderStatusBar(info *StatusBarInfo) string {
	if info == nil {
		return "-- Coding TUI --"
	}
	modified := " "
	if info.Modified {
		modified = "+"
	}
	lang := info.Language
	if lang == "" {
		lang = "code"
	}
	return fmt.Sprintf("[%s]%s %s:%d | %s | %s",
		info.Mode, modified, info.FilePath, info.Position, lang, info.Encoding)
}

// EducationTUIAgent 教育场景 TUI Agent。
// 重写进度显示和状态栏，提供学习进度追踪视图。
type EducationTUIAgent struct {
	*TUIAgentTemplate
}

// NewEducationTUIAgent 创建教育 TUI Agent
func NewEducationTUIAgent() *EducationTUIAgent {
	return &EducationTUIAgent{
		TUIAgentTemplate: NewTUIAgentTemplate(
			AgentEducationTUI,
			"Education TUI Agent",
			"Education-focused terminal user interface agent with learning progress tracking",
		),
	}
}

// RenderStatusBar 教育状态栏：显示学习模式和进度
func (e *EducationTUIAgent) RenderStatusBar(info *StatusBarInfo) string {
	if info == nil {
		return "-- Education TUI --"
	}
	modeLabels := map[string]string{
		"NORMAL":  "学习",
		"INSERT":  "练习",
		"COMMAND": "操作",
	}
	label := modeLabels[info.Mode]
	if label == "" {
		label = info.Mode
	}
	return fmt.Sprintf("[🎓 %s] %s:%d | %s",
		label, info.FilePath, info.Position, info.Encoding)
}

// RenderProgress 教育进度：显示学习完成度和阶段信息
func (e *EducationTUIAgent) RenderProgress(info *ProgressInfo) string {
	if info == nil || info.Total <= 0 {
		return ""
	}
	barWidth := 30
	filled := int(float64(info.Current) / float64(info.Total) * float64(barWidth))
	if filled > barWidth {
		filled = barWidth
	}

	var bar strings.Builder
	for i := 0; i < filled; i++ {
		bar.WriteRune('▬')
	}
	for i := filled; i < barWidth; i++ {
		bar.WriteRune('─')
	}

	pct := float64(info.Current) / float64(info.Total) * 100
	msg := info.Message
	if msg == "" {
		msg = fmt.Sprintf("第 %d/%d 阶段", info.Current, info.Total)
	}
	return fmt.Sprintf("📖 %s %.0f%% %s", bar.String(), pct, msg)
}

// ShowNotification 教育通知：使用更友好的标记
func (e *EducationTUIAgent) ShowNotification(notification *Notification) string {
	if notification == nil {
		return ""
	}
	levelMarkers := map[NotificationLevel]string{
		NotificationInfo:    "📝",
		NotificationSuccess: "✅",
		NotificationWarning: "💡",
		NotificationError:   "❌",
	}
	marker := levelMarkers[notification.Level]
	if marker == "" {
		marker = "•"
	}
	return fmt.Sprintf("%s %s", marker, notification.Message)
}

// Assert interface compliance
var (
	_ TUIAgent = (*TUIAgentTemplate)(nil)
	_ TUIAgent = (*CodingTUIAgent)(nil)
	_ TUIAgent = (*EducationTUIAgent)(nil)
)

// ============================================================================
// TUI Agent 配置模板桩（用于 TemplateRegistry 注册）
// ============================================================================

// tuiConfigTemplate TUI Agent 的 AgentTemplate 桩实现。
// TUI Agent 不生成 CLI 配置文件，仅作为类型占位供注册表识别。
type tuiConfigTemplate struct {
	agentType   AgentType
	displayName string
	description string
}

func newTUIConfigTemplate(agentType AgentType, displayName, description string) *tuiConfigTemplate {
	return &tuiConfigTemplate{
		agentType:   agentType,
		displayName: displayName,
		description: description,
	}
}

func (t *tuiConfigTemplate) AgentType() AgentType               { return t.agentType }
func (t *tuiConfigTemplate) DisplayName() string                { return t.displayName }
func (t *tuiConfigTemplate) Description() string                { return t.description }
func (t *tuiConfigTemplate) Meta() AgentSetupMeta {
	return AgentSetupMeta{
		Category:      AgentCategoryTUI,
		WriteMode:     WriteModeNone,
		ConfigMethod:  "TUI Agent 不通过写入本地配置文件接入 Centag；由进程内路由/流水线绑定。",
		InstallHint:   "内置能力，无需单独安装 CLI",
		AccessMethods: []AccessMethod{AccessBuiltin},
	}
}
func (t *tuiConfigTemplate) ConfigFiles(*BackendInfo) ([]ConfigFile, error) { return nil, nil }
func (t *tuiConfigTemplate) SetupCommand(*BackendInfo) string   { return "" }
func (t *tuiConfigTemplate) PlatformCommands(*BackendInfo) PlatformCommands { return PlatformCommands{} }
func (t *tuiConfigTemplate) VerifyCommand(*BackendInfo) string  { return "" }
func (t *tuiConfigTemplate) Steps(*BackendInfo) []ConfigStep    { return nil }
func (t *tuiConfigTemplate) WriteConfig(*BackendInfo) error     { return nil }

var (
	_ AgentTemplate = (*tuiConfigTemplate)(nil)
)
