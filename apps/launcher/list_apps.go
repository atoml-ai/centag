package main

import (
	"fmt"
	"os"
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
	fmt.Fprintln(tw, "ID\tNAME\tMODE\tINSTALLED\tPATH")
	for _, a := range apps {
		mode := a.LaunchMode
		if mode == "" {
			mode = "wrap_run"
		}
		status, path := "no", ""
		if a.Installed {
			status, path = "yes", a.Path
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", a.ID, a.DisplayName, mode, status, path)
	}
	_ = tw.Flush()

	fmt.Fprintf(os.Stdout, "\n%d/%d installed. Tray: 「代理启动应用」子菜单；CLI: centag wrap run -- <app>\n",
		len(installed), len(apps))
	return nil
}
