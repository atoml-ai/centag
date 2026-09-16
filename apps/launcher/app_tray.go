//go:build tray

package main

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/energye/systray"
)

// Tray icons (trayicon.* asset names kept for compatibility).
//
//go:embed assets/trayicon.ico
var menuIconWindowsICO []byte

//go:embed assets/trayicon.png
var menuIconTemplatePNG []byte

type launcherApp struct {
	cfg Config
	hub *sidecarHub
}

func (a *launcherApp) run(ctx context.Context) error {
	if a.cfg.Headless {
		fmt.Fprintf(os.Stderr, "centag-launcher: sidecar running at %s (headless); press Ctrl+C to stop\n", a.cfg.baseURL())
		<-ctx.Done()
		a.shutdown()
		return nil
	}

	systray.Run(a.onReady, a.onExit)
	return nil
}

func (a *launcherApp) onReady() {
	switch {
	case runtime.GOOS == "darwin" && len(menuIconTemplatePNG) > 0:
		systray.SetTemplateIcon(menuIconTemplatePNG, menuIconTemplatePNG)
	case runtime.GOOS == "windows" && len(menuIconWindowsICO) > 0:
		systray.SetIcon(menuIconWindowsICO)
	case len(menuIconTemplatePNG) > 0:
		systray.SetIcon(menuIconTemplatePNG)
	}

	title := "Centag"
	if a.cfg.Edition == EditionMinimal {
		title = "Centag Minimal"
	}
	systray.SetTooltip(fmt.Sprintf("%s (%s)", title, a.cfg.Edition))
	systray.CreateMenu()

	openItem := systray.AddMenuItem("打开管理界面", "在系统浏览器中打开")
	openItem.Click(func() { _ = openBrowser(a.cfg.baseURL()) })

	runItem := systray.AddMenuItem("代理启动应用", "列出本机已安装、可经 Centag 代理的应用")
	if apps, err := listCatalogAppsCached(a.hub.binary); err != nil {
		fmt.Fprintf(os.Stderr, "centag-launcher: list catalog: %v\n", err)
		mi := runItem.AddSubMenuItem("目录不可用（手动选择程序）", err.Error())
		mi.Click(func() { runAgentViaWrap(a) })
	} else {
		installed := installedApps(apps)
		if len(installed) == 0 {
			runItem.AddSubMenuItem("未检测到已安装应用", "在 Web 管理界面查看安装指引")
		} else {
			for _, app := range installed {
				app := app
				if needsManualAgentConfig(app) {
					mi := runItem.AddSubMenuItem(app.DisplayName, "⚠️ 无法自动代理，需手动配置 Agent")
					mi.Click(func() { showManualAgentConfigDialog(a, app) })
				} else {
					mi := runItem.AddSubMenuItem(app.DisplayName, "经 centag 代理启动 "+app.ID)
					mi.Click(func() { runAgentByApp(a, app) })
				}
			}
		}
	}
	manualItem := runItem.AddSubMenuItem("手动选择程序…", "选择任意本机程序并代理启动")
	manualItem.Click(func() { runAgentViaWrap(a) })

	writeCfgItem := systray.AddMenuItemCheckbox("启动前写入模型配置",
		"将应用模型写为 centag/默认流水线（可选；默认依赖透明模式映射）", writeConfigOnLaunch)
	writeCfgItem.Click(func() {
		writeConfigOnLaunch = !writeConfigOnLaunch
		if writeConfigOnLaunch {
			writeCfgItem.Check()
		} else {
			writeCfgItem.Uncheck()
		}
	})

	doctorItem := systray.AddMenuItem("代理诊断", "检查 CA / MITM / 出口 Key 等就绪状态")
	doctorItem.Click(func() { runDoctorNow(a) })

	cliItem := systray.AddMenuItem("安装命令行工具", "将 centag 命令安装到 PATH（终端可用 centag wrap）")
	cliItem.Click(func() {
		if err := installCentagCLI(a.hub.binary); err != nil {
			fmt.Fprintf(os.Stderr, "centag-launcher: install cli failed: %v\n", err)
			notifyUser("Centag", "命令行安装失败: "+err.Error())
			return
		}
		notifyUser("Centag", "centag 命令已安装，终端可直接使用 centag wrap")
	})

	trustItem := systray.AddMenuItem("信任 CA 证书", "将 Centag CA 安装到系统钥匙串（被代理应用信任 MITM 证书，一次即可）")
	trustItem.Click(func() {
		caPath, err := trustCACert(a.cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "centag-launcher: trust ca failed: %v\n", err)
			notifyUser("Centag", "CA 信任失败: "+err.Error())
			return
		}
		if caPath == "" {
			return // user cancelled the authorization dialog
		}
		notifyUser("Centag", "Centag CA 已安装到系统钥匙串，被代理应用即可发起 HTTPS 请求")
	})

	untrustItem := systray.AddMenuItem("移除 CA 信任", "从系统钥匙串移除 Centag CA（不再信任 Centag 的 MITM 证书）")
	untrustItem.Click(func() {
		if err := untrustCACert(a.cfg); err != nil {
			fmt.Fprintf(os.Stderr, "centag-launcher: untrust ca failed: %v\n", err)
			notifyUser("Centag", "CA 移除失败: "+err.Error())
			return
		}
		notifyUser("Centag", "Centag CA 已从系统钥匙串移除")
	})

	systray.AddSeparator()

	quitItem := systray.AddMenuItem("退出", "停止 sidecar 并退出")
	quitItem.Click(func() {
		a.shutdown()
		systray.Quit()
	})

	systray.SetOnDClick(func(systray.IMenu) {
		_ = openBrowser(a.cfg.baseURL())
	})
}

func (a *launcherApp) onExit() {
	a.shutdown()
}

func (a *launcherApp) shutdown() {
	if a.hub != nil {
		a.hub.shutdown()
	}
}

func quitMenu(enabled bool) {
	if enabled {
		systray.Quit()
	}
}

// needsManualAgentConfig checks if the app needs manual agent configuration
// because its LLM requests use certificate pinning and cannot be proxied via MITM.
func needsManualAgentConfig(app catalogApp) bool {
	id := strings.ToLower(app.ID)
	name := strings.ToLower(app.DisplayName)
	// WorkBuddy/CodeBuddy: LLM requests use certificate pinning
	return strings.Contains(id, "workbuddy") || strings.Contains(id, "codebuddy") ||
		strings.Contains(name, "workbuddy") || strings.Contains(name, "codebuddy")
}

// showManualAgentConfigDialog shows a dialog guiding users to configure the agent
// through centag's backend/model configuration instead of HTTP proxy.
func showManualAgentConfigDialog(a *launcherApp, app catalogApp) {
	msg := fmt.Sprintf(`%s 的 LLM 请求使用了证书固定 (Certificate Pinning)，
无法通过 Centag MITM 代理自动拦截。

请使用 Centag 的 Agent 配置能力手动接入：

1. 打开 %s → 设置 → 模型 → 自定义 API
2. 填写以下参数：
   - 请求地址: http://127.0.0.1:20060/v1
   - 模型 ID: centag/<流水线名称>
   - API Key: <Centag API Key>

3. 保存后在对话中选择 Centag 模型即可

详细文档: https://www.codebuddy.ai/docs/zh/workbuddy/From-Beginner-to-Expert-Guide/Function-Description/Model

是否打开设置页面？`, app.DisplayName, app.DisplayName)

	switch runtime.GOOS {
	case "darwin":
		script := fmt.Sprintf(`display dialog "%s" with title "Centag - Agent 配置指引" buttons {"取消", "打开设置"} default button "打开设置"`, strings.ReplaceAll(msg, `"`, `\"`))
		out, err := runCommand(30*time.Second, "osascript", "-e", script)
		if err == nil && strings.Contains(out, "打开设置") {
			// Open the doc URL
			_, _ = runCommand(5*time.Second, "open", "https://www.codebuddy.ai/docs/zh/workbuddy/From-Beginner-to-Expert-Guide/Function-Description/Model")
		}
	case "windows":
		notifyUser("Centag - Agent 配置指引",
			fmt.Sprintf("%s 需手动配置 Agent。请在设置 → 模型 → 自定义 API 中添加 centag 后端。", app.DisplayName))
	}
}
