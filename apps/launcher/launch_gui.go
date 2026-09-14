package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// errNoPerProcessProxy means the app cannot be given a per-process proxy. We do
// NOT fall back to the OS system proxy: writing system-wide proxy settings is
// invasive and affects every other application, so the launch fails with
// guidance instead.
var errNoPerProcessProxy = errors.New("应用不支持进程级代理")

// launchSystemProxyApp opens a GUI app with Centag egress using ONLY per-process
// techniques (Chromium --proxy-server flags). It never modifies the OS system
// proxy: apps without a per-process handle fail with guidance toward app-level
// or manual configuration.
func launchSystemProxyApp(a *launcherApp, app catalogApp) error {
	if !app.Install.WinChromium {
		return noProcessProxyError(app)
	}
	if err := installCAOnly(a); err != nil {
		return err
	}
	switch err := openAppChromium(app); {
	case err == nil:
		return nil
	case errors.Is(err, errNoPerProcessProxy):
		return noProcessProxyError(app)
	default:
		return err
	}
}

// noProcessProxyError explains how to route the app through Centag without
// touching OS-wide proxy settings.
func noProcessProxyError(app catalogApp) error {
	name := strings.TrimSpace(app.DisplayName)
	if name == "" {
		name = app.ID
	}
	return fmt.Errorf(
		"%w：%s 无法用进程级代理接管，且 Centag 不会改动系统代理。"+
			"请任选其一解决：\n"+
			"  1) 在该应用的设置/配置文件中把模型 API 指向 Centag（写入其配置文件）；\n"+
			"  2) 手动把该应用的代理设置为 Centag MITM 地址；\n"+
			"  3) 命令行的应用可用 `centag wrap run -- <可执行文件>` 启动",
		errNoPerProcessProxy, name)
}

// installCAOnly installs the Centag CA (needed only for MITM'd whitelisted
// domains) without writing the OS system proxy.
func installCAOnly(a *launcherApp) error {
	out, err := runWrapEnable(a, false, "--ca-only")
	if err != nil {
		return fmt.Errorf("wrap enable --ca-only: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// runWrapEnable invokes the sidecar CLI; extra flags (e.g. --ca-only) are
// appended after the optional --force re-assert flag.
func runWrapEnable(a *launcherApp, force bool, extra ...string) ([]byte, error) {
	cmd := exec.Command(a.hub.binary, wrapEnableArgs(force, extra...)...)
	cmd.Env = append(os.Environ(), "CENTAG_API_BASE="+a.cfg.baseURL())
	if tok := strings.TrimSpace(os.Getenv("CENTAG_WRAP_TOKEN")); tok != "" {
		cmd.Env = append(cmd.Env, "CENTAG_WRAP_TOKEN="+tok)
	}
	hideSidecarWindow(cmd)
	return cmd.CombinedOutput()
}

// wrapEnableArgs builds the `wrap enable` argv: an optional --force re-assert
// followed by extra flags (e.g. --ca-only for the system-proxy-free GUI path).
func wrapEnableArgs(force bool, extra ...string) []string {
	args := []string{"wrap", "enable"}
	if force {
		args = append(args, "--force")
	}
	return append(args, extra...)
}
