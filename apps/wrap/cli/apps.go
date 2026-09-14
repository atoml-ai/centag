package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"text/tabwriter"
	"time"
)

const localAPIBase = "http://127.0.0.1:20060"

// appCatalogJSON is injected by the embedding binary (centag core registry)
// so `centag wrap apps` works offline. Standalone centag-wrap leaves it nil
// and falls back to the local sidecar HTTP API.
var appCatalogJSON func() ([]byte, error)

// SetAppCatalogJSON registers an offline catalog provider. Call from main/init
// when the distribution embeds the Agent registry (personal/minimal builds).
func SetAppCatalogJSON(fn func() ([]byte, error)) {
	appCatalogJSON = fn
}

type installSpec struct {
	CLIBinaries []string `json:"cli_binaries,omitempty"`
	MacApps     []string `json:"mac_apps,omitempty"`
	WinExes     []string `json:"win_exes,omitempty"`
	WinMSIX     []string `json:"win_msix,omitempty"`
	WinChromium bool     `json:"win_chromium,omitempty"`
	Aliases     []string `json:"aliases,omitempty"`
}

type wrapApp struct {
	ID          string      `json:"id"`
	DisplayName string      `json:"display_name"`
	Vendor      string      `json:"vendor,omitempty"`
	Category    string      `json:"category,omitempty"`
	LaunchMode  string      `json:"launch_mode"`
	Argv        []string    `json:"argv,omitempty"`
	Install     installSpec `json:"install"`
	InstallURL  string      `json:"install_url,omitempty"`
	InstallHint string      `json:"install_hint,omitempty"`
	Note        string      `json:"note,omitempty"`
}

type wrapCatalog struct {
	Apps []wrapApp `json:"apps"`
}

// appRow augments a catalog entry with local install detection.
type appRow struct {
	wrapApp
	Installed bool   `json:"installed"`
	Path      string `json:"path,omitempty"`
}

// runApps implements `centag wrap apps`: list apps that can be launched through
// Centag wrap, with best-effort local install detection.
func runApps(args []string) error {
	return runAppsTo(os.Stdout, args)
}

func runAppsTo(w io.Writer, args []string) error {
	var jsonOut, installedOnly bool
	var server, token string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--json":
			jsonOut = true
		case a == "--installed":
			installedOnly = true
		case a == "--server" || a == "-s":
			if i+1 >= len(args) {
				return fmt.Errorf("%s requires a value", a)
			}
			i++
			server = strings.TrimSpace(args[i])
		case strings.HasPrefix(a, "--server="):
			server = strings.TrimSpace(strings.TrimPrefix(a, "--server="))
		case a == "--token" || a == "-t":
			if i+1 >= len(args) {
				return fmt.Errorf("%s requires a value", a)
			}
			i++
			token = strings.TrimSpace(args[i])
		case strings.HasPrefix(a, "--token="):
			token = strings.TrimSpace(strings.TrimPrefix(a, "--token="))
		case a == "--help" || a == "-h":
			return nil
		default:
			return fmt.Errorf("unknown flag %q", a)
		}
	}

	cat, err := loadCatalog(server, token)
	if err != nil {
		return err
	}

	rows := make([]appRow, 0, len(cat.Apps))
	for _, app := range cat.Apps {
		path, ok := detectInstalled(app.Install)
		if installedOnly && !ok {
			continue
		}
		rows = append(rows, appRow{wrapApp: app, Installed: ok, Path: path})
	}

	if jsonOut {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]any{"apps": rows})
	}
	printAppTable(w, rows)
	return nil
}

// loadCatalog resolves the catalog from the injected provider, or falls back to
// the local sidecar HTTP API.
func loadCatalog(server, token string) (wrapCatalog, error) {
	var cat wrapCatalog
	if appCatalogJSON != nil {
		payload, err := appCatalogJSON()
		if err != nil {
			return cat, fmt.Errorf("build app catalog: %w", err)
		}
		if err := json.Unmarshal(payload, &cat); err != nil {
			return cat, fmt.Errorf("decode app catalog: %w", err)
		}
		return cat, nil
	}

	base := strings.TrimSpace(server)
	if base == "" {
		base = strings.TrimSpace(os.Getenv("CENTAG_API_BASE"))
	}
	if base == "" {
		base = localAPIBase
	}
	if !strings.Contains(base, "://") {
		base = "http://" + base
	}
	base = strings.TrimRight(base, "/")

	req, err := http.NewRequest(http.MethodGet, base+"/api/v1/wrap/apps", nil)
	if err != nil {
		return cat, err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return cat, fmt.Errorf("fetch %s/api/v1/wrap/apps: %w (run via `centag wrap apps` for the offline catalog)", base, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return cat, fmt.Errorf("app catalog HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if err := json.Unmarshal(body, &cat); err != nil {
		return cat, fmt.Errorf("decode app catalog: %w", err)
	}
	return cat, nil
}

// detectInstalled checks the install spec against the local machine.
// Returns the first resolved path. Pure stdlib; no cgo.
func detectInstalled(spec installSpec) (string, bool) {
	for _, bin := range spec.CLIBinaries {
		if bin == "" {
			continue
		}
		if p, err := exec.LookPath(bin); err == nil {
			return p, true
		}
	}
	// Desktop GUI apps: match aliases against the installed-app index
	// (Windows Start Menu / macOS /Applications), no hardcoded paths.
	if p, ok := matchInstalledApp(spec.Aliases); ok {
		return p, true
	}
	switch runtime.GOOS {
	case "darwin":
		home, _ := os.UserHomeDir()
		dirs := []string{"/Applications"}
		if home != "" {
			dirs = append(dirs, filepath.Join(home, "Applications"))
		}
		for _, name := range spec.MacApps {
			for _, dir := range dirs {
				p := filepath.Join(dir, name+".app")
				if st, err := os.Stat(p); err == nil && st.IsDir() {
					return p, true
				}
			}
		}
	case "windows":
		// Windows 商店应用（MSIX）：在 %LOCALAPPDATA%\Packages 下按
		// PackageFamilyName 前缀匹配；路径返回 AUMID，供启动器
		// `explorer shell:AppsFolder\<AUMID>` 打开。
		if p, ok := detectWinMSIX(spec.WinMSIX); ok {
			return p, true
		}
		for _, exe := range spec.WinExes {
			if exe == "" {
				continue
			}
			if p, err := exec.LookPath(exe); err == nil {
				return p, true
			}
			for _, dir := range winSearchDirs() {
				p := filepath.Join(dir, exe)
				if st, err := os.Stat(p); err == nil && !st.IsDir() {
					return p, true
				}
			}
		}
	}
	return "", false
}

// winMSIXPackagesDir points the MSIX probe at %LOCALAPPDATA%\Packages
// (injectable for tests).
var winMSIXPackagesDir = func() string {
	local := strings.TrimSpace(os.Getenv("LOCALAPPDATA"))
	if local == "" {
		return ""
	}
	return filepath.Join(local, "Packages")
}

// detectWinMSIX matches a PackageFamilyName prefix against the per-user MSIX
// package store. Returns the AUMID (`<package family>!App`) when found.
func detectWinMSIX(prefixed []string) (string, bool) {
	root := winMSIXPackagesDir()
	if root == "" {
		return "", false
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return "", false
	}
	for _, prefix := range prefixed {
		prefix = strings.ToLower(strings.TrimSpace(prefix))
		if prefix == "" {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			name := e.Name()
			if !strings.Contains(name, "_") || !strings.HasPrefix(strings.ToLower(name), prefix) {
				continue
			}
			return name + "!App", true
		}
	}
	return "", false
}

// winSearchDirs lists common install roots for Windows executables.
func winSearchDirs() []string {
	var dirs []string
	for _, env := range []string{"ProgramFiles", "ProgramFiles(x86)", "ProgramData", "LOCALAPPDATA"} {
		if v := strings.TrimSpace(os.Getenv(env)); v != "" {
			dirs = append(dirs, v)
		}
	}
	if local := strings.TrimSpace(os.Getenv("LOCALAPPDATA")); local != "" {
		dirs = append(dirs, filepath.Join(local, "Programs"))
	}
	return dirs
}

func printAppTable(w io.Writer, rows []appRow) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tNAME\tMODE\tINSTALLED\tPATH")
	installed := 0
	for _, r := range rows {
		status := "no"
		path := ""
		if r.Installed {
			status = "yes"
			path = r.Path
			installed++
		}
		mode := r.LaunchMode
		if mode == "" {
			mode = "wrap_run"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", r.ID, r.DisplayName, mode, status, path)
	}
	_ = tw.Flush()
	fmt.Fprintf(w, "\n%d/%d installed. CLI/TUI: `centag wrap run -- <app>`; GUI: 系统 PAC 启动（tray）\n", installed, len(rows))
}
