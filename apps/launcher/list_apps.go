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
		if isWorkBuddyCatalogApp(a) {
			notes = "⚠️ LLM需手动配置代理"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", a.ID, a.DisplayName, mode, status, path, notes)
	}
	_ = tw.Flush()

	fmt.Fprintf(os.Stdout, "\n%d/%d installed. Tray: 「代理启动应用」子菜单；CLI: centag wrap run -- <app>\n",
		len(installed), len(apps))
	return nil
}

// isWorkBuddyCatalogApp checks if the catalog app is WorkBuddy/CodeBuddy
func isWorkBuddyCatalogApp(app catalogApp) bool {
	id := strings.ToLower(app.ID)
	name := strings.ToLower(app.DisplayName)
	return strings.Contains(id, "workbuddy") || strings.Contains(id, "codebuddy") ||
		strings.Contains(name, "workbuddy") || strings.Contains(name, "codebuddy")
}
