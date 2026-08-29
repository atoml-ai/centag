//go:build tray

package main

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"runtime"

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

	runItem := systray.AddMenuItem("运行", "用 wrap 启动 Agent 应用")
	runItem.Click(func() { _ = openBrowser(a.cfg.baseURL() + "/agent-run") })

	cliItem := systray.AddMenuItem("安装命令行工具", "将 centag 命令安装到 PATH（终端可用 centag wrap）")
	cliItem.Click(func() {
		if err := installCentagCLI(a.hub.binary); err != nil {
			fmt.Fprintf(os.Stderr, "centag-launcher: install cli failed: %v\n", err)
			notifyUser("Centag", "命令行安装失败: "+err.Error())
			return
		}
		notifyUser("Centag", "centag 命令已安装，终端可直接使用 centag wrap")
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
