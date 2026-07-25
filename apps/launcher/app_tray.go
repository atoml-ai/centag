//go:build tray

package main

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"runtime"
	"sync"

	"github.com/energye/systray"
)

// Tray icons (trayicon.* asset names kept for compatibility).
//
//go:embed assets/trayicon.ico
var menuIconWindowsICO []byte

//go:embed assets/trayicon.png
var menuIconTemplatePNG []byte

type launcherApp struct {
	cfg     Config
	sidecar *sidecarProcess
	mu      sync.Mutex
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
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.sidecar != nil {
		_ = a.sidecar.stop()
		a.sidecar = nil
	}
}

func quitMenu(enabled bool) {
	if enabled {
		systray.Quit()
	}
}
