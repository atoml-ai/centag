package agent

import "strings"

// AccessMethod Agent 接入方式（驱动 UI，禁止前端按 category 猜测）。
type AccessMethod string

const (
	// AccessWriteConfig 改本地配置文件，使请求走 Centag 代理。
	AccessWriteConfig AccessMethod = "write_config"
	// AccessUIGuide 必须在客户端 UI 中配置（无有效本地写配置）。
	AccessUIGuide AccessMethod = "ui_guide"
	// AccessWrapCLI 通过 centag wrap run 启动配套 CLI。
	AccessWrapCLI AccessMethod = "wrap_cli"
	// AccessBuiltin 内置 TUI/Web，进程内路由/流水线绑定。
	AccessBuiltin AccessMethod = "builtin"
)

// CompanionCLI wrap 启动的配套命令行（不是桌面 .app launcher）。
type CompanionCLI struct {
	Binary      string   `json:"binary"`
	Argv        []string `json:"argv,omitempty"`
	InstallURL  string   `json:"install_url,omitempty"`
	InstallHint string   `json:"install_hint,omitempty"`
	Note        string   `json:"note,omitempty"`
}

// UIGuideField UI 配置表单项（可复制参数）。
type UIGuideField struct {
	Label string `json:"label"`
	Value string `json:"value"`
	Hint  string `json:"hint,omitempty"`
}

// UI 指引中的请求地址形态。
const (
	// RequestURLOpenAIBase OpenAI 兼容 base：…/v1（客户端自行拼接 /chat/completions）。
	RequestURLOpenAIBase = "openai_base"
	// RequestURLChatCompletions 完整 chat 端点：…/v1/chat/completions。
	RequestURLChatCompletions = "chat_completions"
)

// UIGuide 桌面/客户端内手动配置指引。
type UIGuide struct {
	Title   string `json:"title,omitempty"`
	Summary string `json:"summary,omitempty"`
	DocURL  string `json:"doc_url,omitempty"`
	Steps   []string `json:"steps,omitempty"`
	Fields  []UIGuideField `json:"fields,omitempty"`
	// RequestURLKind 向导「请求地址」应复制的 URL 形态；空则默认 openai_base（…/v1）。
	// UI 指引统一默认 openai_base；个别客户端若也接受完整路径，用 URLHint 说明即可。
	RequestURLKind string `json:"request_url_kind,omitempty"`
	// FullURLMode 客户端「完整 URL」开关：on / off / 空（不展示该行）。
	FullURLMode string `json:"full_url_mode,omitempty"`
	// URLHint 请求地址旁的补充说明（如「也可填 …/chat/completions」）。
	URLHint     string `json:"url_hint,omitempty"`
	ExportHint  string `json:"export_hint,omitempty"`
	RestartHint string `json:"restart_hint,omitempty"`
}

// ResolveRequestURL 按 RequestURLKind 生成应填入客户端的地址。
func ResolveRequestURL(kind, host string, port int) string {
	switch kind {
	case RequestURLChatCompletions:
		return chatCompletionsURL(host, port)
	default:
		return proxyURL(host, port)
	}
}

// WrapLaunchInfo 可由 wrap 启动的 Agent 目标（供 server 层列出 preset）。
type WrapLaunchInfo struct {
	ID          string
	DisplayName string
	Description string
	Argv        []string
}

// HasAccess 是否声明了指定接入方式。
func (m AgentSetupMeta) HasAccess(method AccessMethod) bool {
	for _, a := range m.AccessMethods {
		if a == method {
			return true
		}
	}
	return false
}

// AnyVerified 任一接入方式已验证（用于排序/卡片高亮）。
func (m AgentSetupMeta) AnyVerified() bool {
	return m.VerifiedWrite || m.VerifiedWrap || m.VerifiedUI
}

// WrapArgv 返回 wrap 用 argv；无 companion 时返回 nil。
func (m AgentSetupMeta) WrapArgv() []string {
	if m.CompanionCLI == nil {
		return nil
	}
	if len(m.CompanionCLI.Argv) > 0 {
		return append([]string(nil), m.CompanionCLI.Argv...)
	}
	bin := strings.TrimSpace(m.CompanionCLI.Binary)
	if bin == "" {
		return nil
	}
	return []string{bin}
}

// Normalize 补齐 AccessMethods / CompanionCLI.Argv 等派生字段。
// 若模板未显式填写 AccessMethods，则按 WriteMode + CompanionCLI 兼容推导。
func (m AgentSetupMeta) Normalize() AgentSetupMeta {
	out := m
	if out.CompanionCLI != nil {
		cli := *out.CompanionCLI
		cli.Binary = strings.TrimSpace(cli.Binary)
		if len(cli.Argv) == 0 && cli.Binary != "" {
			cli.Argv = []string{cli.Binary}
		}
		out.CompanionCLI = &cli
	}

	if len(out.AccessMethods) == 0 {
		switch {
		case out.WriteMode == WriteModeNone && out.Category == AgentCategoryTUI:
			out.AccessMethods = []AccessMethod{AccessBuiltin}
		case out.WriteMode == WriteModeNone && out.Category == AgentCategoryWeb:
			out.AccessMethods = []AccessMethod{AccessBuiltin}
		case out.WriteMode == WriteModeNone && out.UIGuide != nil:
			out.AccessMethods = []AccessMethod{AccessUIGuide}
		case out.WriteMode == WriteModeMerge || out.WriteMode == WriteModeOverwrite:
			out.AccessMethods = []AccessMethod{AccessWriteConfig}
		}
		if out.CompanionCLI != nil && strings.TrimSpace(out.CompanionCLI.Binary) != "" {
			if !out.HasAccess(AccessWrapCLI) {
				out.AccessMethods = append(out.AccessMethods, AccessWrapCLI)
			}
		}
	}

	// 声明了 wrap_cli 但未填 Binary 时去掉无效方式
	if out.HasAccess(AccessWrapCLI) && (out.CompanionCLI == nil || strings.TrimSpace(out.CompanionCLI.Binary) == "") {
		filtered := make([]AccessMethod, 0, len(out.AccessMethods))
		for _, a := range out.AccessMethods {
			if a != AccessWrapCLI {
				filtered = append(filtered, a)
			}
		}
		out.AccessMethods = filtered
	}

	return out
}

// NewCLICompanion 构建纯 CLI 的 CompanionCLI。
func NewCLICompanion(binary, installURL, installHint string) *CompanionCLI {
	return &CompanionCLI{
		Binary:      binary,
		Argv:        []string{binary},
		InstallURL:  installURL,
		InstallHint: installHint,
	}
}

// NewDesktopCompanionCLI 桌面产品配套 CLI（wrap 不用 .app）。
func NewDesktopCompanionCLI(binary, installURL, installHint string) *CompanionCLI {
	return &CompanionCLI{
		Binary:      binary,
		Argv:        []string{binary},
		InstallURL:  installURL,
		InstallHint: installHint,
		Note:        "wrap 启动配套 CLI，不是桌面 .app",
	}
}

// WrapLaunchTargets 从注册表收集声明了 wrap_cli 的 Agent。
func WrapLaunchTargets(r *TemplateRegistry) []WrapLaunchInfo {
	if r == nil {
		return nil
	}
	var out []WrapLaunchInfo
	for _, at := range r.List() {
		tmpl, ok := r.Get(at)
		if !ok {
			continue
		}
		meta := tmpl.Meta().Normalize()
		if !meta.HasAccess(AccessWrapCLI) {
			continue
		}
		argv := meta.WrapArgv()
		if len(argv) == 0 {
			continue
		}
		out = append(out, WrapLaunchInfo{
			ID:          string(at),
			DisplayName: tmpl.DisplayName(),
			Description: tmpl.Description(),
			Argv:        argv,
		})
	}
	return out
}

// GuideOnly 仅 UI 指引、无真正 write_config（可导出说明文件）。
func (m AgentSetupMeta) GuideOnly() bool {
	n := m.Normalize()
	return n.HasAccess(AccessUIGuide) && !n.HasAccess(AccessWriteConfig)
}
