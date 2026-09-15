package agent

import (
	"sort"
	"strings"
)

// LaunchMode 应用经 Centag 代理启动的方式。
type LaunchMode string

const (
	// LaunchWrapRun 进程级代理：等价 `centag wrap run`（CLI 类，不修改系统代理）。
	LaunchWrapRun LaunchMode = "wrap_run"
	// LaunchSystemProxy 进程级代理（桌面 GUI 类）：信任 CA + 进程级代理注入启动。
	// 不修改系统代理，仅影响该应用的大模型服务访问。
	LaunchSystemProxy LaunchMode = "system_proxy"
)

// InstallSpec 本机安装检测线索（尽力而为，纯标准库）。
type InstallSpec struct {
	CLIBinaries []string `json:"cli_binaries,omitempty"`
	MacApps     []string `json:"mac_apps,omitempty"`
	WinExes     []string `json:"win_exes,omitempty"`
	// Aliases 用于「已安装应用索引」模糊匹配（Windows 开始菜单/注册表、
	// macOS /Applications），适合不硬编码路径的桌面应用。
	Aliases []string `json:"aliases,omitempty"`
	// WinMSIX Windows 商店应用（MSIX）的 PackageFamilyName 前缀。
	WinMSIX []string `json:"win_msix,omitempty"`
	// WinChromium Chromium 内核标记：启动时注入 --proxy-server=<MITM>。
	WinChromium bool `json:"win_chromium,omitempty"`
}

// ModelConfig 让应用使用 Centag 模型名的方式（M1 仅携带，启动注入在后续里程碑）。
type ModelConfig struct {
	// AgentType 非空则复用该 Agent 模板 WriteConfig 写入模型（centag/<pipeline> 等）。
	AgentType string `json:"agent_type,omitempty"`
	// EnvKeys 无本地写配置能力时，wrap 启动前注入的模型环境变量名。
	EnvKeys []string `json:"env_keys,omitempty"`
	// Requires 是否建议使用 Centag 模型名才可稳定工作（供 UI 提示）。
	Requires bool `json:"requires,omitempty"`
}

// ProxyApp 可经 Centag 代理启动的目录项（供 wrap CLI / desktop / Web 消费）。
type ProxyApp struct {
	ID          string        `json:"id"`
	DisplayName string        `json:"display_name"`
	Vendor      string        `json:"vendor,omitempty"`
	Category    AgentCategory `json:"category"`
	LaunchMode  LaunchMode    `json:"launch_mode"`
	Argv        []string      `json:"argv,omitempty"`
	Install     InstallSpec   `json:"install,omitzero"`
	InstallURL  string        `json:"install_url,omitempty"`
	InstallHint string        `json:"install_hint,omitempty"`
	ModelConfig ModelConfig   `json:"model_config,omitzero"`
	Verified    bool          `json:"verified,omitempty"`
	Note        string        `json:"note,omitempty"`
}

// ProxyApps 从注册表派生「可经 Centag 代理启动」的目录。
//
// M1 仅派生声明了 wrap_cli 的目标（CompanionCLI），安装检测线索取 CLI 二进制名；
// 桌面 GUI（system_proxy）需要 per-app 安装检测规格，留待后续里程碑。
// 返回按 Vendor + ID 稳定排序，便于 UI/CLI 展示。
func ProxyApps(r *TemplateRegistry) []ProxyApp {
	if r == nil {
		return nil
	}
	out := make([]ProxyApp, 0, len(r.List()))
	for _, at := range r.List() {
		t, ok := r.Get(at)
		if !ok {
			continue
		}
		meta := t.Meta().Normalize()
		if !meta.HasAccess(AccessWrapCLI) {
			continue
		}
		argv := meta.WrapArgv()
		if len(argv) == 0 {
			continue
		}
		bin := argv[0]
		if meta.CompanionCLI != nil && meta.CompanionCLI.Binary != "" {
			bin = meta.CompanionCLI.Binary
		}
		app := ProxyApp{
			ID:          string(at),
			DisplayName: t.DisplayName(),
			Vendor:      meta.Vendor,
			Category:    meta.Category,
			LaunchMode:  LaunchWrapRun,
			Argv:        argv,
			Install:     InstallSpec{CLIBinaries: []string{bin}},
			InstallURL:  meta.InstallURL,
			InstallHint: meta.InstallHint,
			Verified:    meta.VerifiedWrap,
		}
		if meta.HasAccess(AccessWriteConfig) {
			app.ModelConfig = ModelConfig{AgentType: string(at), Requires: true}
		}
		if meta.CompanionCLI != nil {
			app.Note = meta.CompanionCLI.Note
		}
		out = append(out, app)
	}

	// Desktop GUI editions are listed separately (CLI/TUI and desktop are
	// distinct targets, launched differently: terminal vs system proxy).
	for _, at := range r.List() {
		t, ok := r.Get(at)
		if !ok {
			continue
		}
		meta := t.Meta().Normalize()
		for _, ed := range meta.DesktopEditions {
			id := strings.TrimSpace(ed.ID)
			if id == "" || strings.TrimSpace(ed.DisplayName) == "" {
				continue
			}
			aliases := append([]string(nil), ed.Aliases...)
			if len(aliases) == 0 {
				aliases = append(aliases, ed.MacApps...)
				aliases = append(aliases, ed.WinExes...)
			}
			out = append(out, ProxyApp{
				ID:          id,
				DisplayName: ed.DisplayName,
				Vendor:      meta.Vendor,
				Category:    AgentCategoryDesktop,
				LaunchMode:  LaunchSystemProxy,
				Install: InstallSpec{
					MacApps: ed.MacApps,
					WinExes: ed.WinExes,
					Aliases: aliases,
					WinMSIX: ed.WinMSIX,
					WinChromium: ed.WinChromium,
				},
				InstallURL:  meta.InstallURL,
				InstallHint: meta.InstallHint,
				Verified:    meta.VerifiedWrap,
				Note:        ed.Note,
			})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Vendor != out[j].Vendor {
			return out[i].Vendor < out[j].Vendor
		}
		return out[i].ID < out[j].ID
	})
	return out
}
