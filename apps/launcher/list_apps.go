package main

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
)

// runListApps implements `centag-desktop --list-apps`: print the proxy-launch
// catalog with local install detection, then exit (no sidecar, no tray). Used
// for verification and troubleshooting.
func runListApps(binary string) error {
	apps, err := listCatalogApps(binary)
	if err != nil {
		return err
	}
	installed := installedApps(apps)

	tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tNAME\tMODE\tINSTALLED\tPATH\tNOTES")
	for _, a := range apps {
		mode := a.LaunchMode
		if mode == "" {
			mode = "wrap_run"
		}
		status, path := "no", ""
		if a.Installed {
			status, path = "yes", a.Path
		}
		notes := ""
		if needsManualAgentConfigCatalog(a) {
			notes = "⚠️ 需手动配置 Agent"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", a.ID, a.DisplayName, mode, status, path, notes)
	}
	_ = tw.Flush()

	fmt.Fprintf(os.Stdout, "\n%d/%d installed. Tray: 「代理启动应用」子菜单；CLI: centag wrap run -- <app>\n",
		len(installed), len(apps))

	// Show manual agent config warning for apps that need it
	for _, a := range installed {
		if needsManualAgentConfigCatalog(a) {
			fmt.Fprintf(os.Stdout, "\n⚠️  %s 配置指引:\n", a.DisplayName)
			fmt.Fprintf(os.Stdout, "    该应用的 LLM 请求使用证书固定，无法通过 MITM 代理自动拦截。\n")
			fmt.Fprintf(os.Stdout, "    请使用 Centag 的 Agent 配置能力手动接入：\n")
			fmt.Fprintf(os.Stdout, "    1. 打开 %s → 设置 → 模型 → 自定义 API\n", a.DisplayName)
			fmt.Fprintf(os.Stdout, "    2. 请求地址: http://127.0.0.1:20060/v1\n")
			fmt.Fprintf(os.Stdout, "    3. 模型 ID: centag/<流水线名称>\n")
			fmt.Fprintf(os.Stdout, "    4. API Key: <Centag API Key>\n")
			fmt.Fprintf(os.Stdout, "    文档: https://www.codebuddy.ai/docs/zh/workbuddy/From-Beginner-to-Expert-Guide/Function-Description/Model\n")
			break
		}
	}

	return nil
}

// needsManualAgentConfigCatalog checks if the catalog app needs manual agent configuration
func needsManualAgentConfigCatalog(app catalogApp) bool {
	id := strings.ToLower(app.ID)
	name := strings.ToLower(app.DisplayName)
	return strings.Contains(id, "workbuddy") || strings.Contains(id, "codebuddy") ||
		strings.Contains(name, "workbuddy") || strings.Contains(name, "codebuddy")
}
