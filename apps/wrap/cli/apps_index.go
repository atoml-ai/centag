package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// installedApp 是本机「已安装应用索引」的一项，用于桌面 GUI 应用的别名匹配检测。
type installedApp struct {
	Name string
	Path string
}

var (
	appIndexOnce sync.Once
	appIndex     []installedApp
)

// installedAppIndex 枚举本机应用（Windows 开始菜单 .lnk / macOS /Applications），
// 进程内缓存一次。纯标准库 + 系统自带工具，无 cgo / 新依赖。
func installedAppIndex() []installedApp {
	appIndexOnce.Do(func() {
		switch runtime.GOOS {
		case "windows":
			appIndex = windowsStartMenuApps()
		case "darwin":
			appIndex = macApplications()
		}
	})
	return appIndex
}

// appIndexProvider 是索引来源的注入点（测试可覆盖）。
var appIndexProvider = installedAppIndex

// matchInstalledApp 按别名（大小写不敏感、子串匹配）在已安装应用索引中查找。
func matchInstalledApp(aliases []string) (string, bool) {
	apps := appIndexProvider()
	for _, a := range apps {
		name := strings.ToLower(a.Name)
		for _, al := range aliases {
			al = strings.ToLower(strings.TrimSpace(al))
			if al != "" && strings.Contains(name, al) {
				return a.Path, true
			}
		}
	}
	return "", false
}

const windowsStartMenuScript = `$ErrorActionPreference = 'SilentlyContinue'
$sh = New-Object -ComObject WScript.Shell
$dirs = @(
  (Join-Path $env:ProgramData 'Microsoft\Windows\Start Menu\Programs'),
  (Join-Path $env:APPDATA 'Microsoft\Windows\Start Menu\Programs')
)
$seen = @{}
foreach ($d in $dirs) {
  if (-not (Test-Path $d)) { continue }
  Get-ChildItem -Path $d -Filter *.lnk -Recurse -ErrorAction SilentlyContinue | ForEach-Object {
    $key = $_.BaseName.ToLower()
    if ($seen.ContainsKey($key)) { return }
    $seen[$key] = $true
    $t = ''
    try { $t = $sh.CreateShortcut($_.FullName).TargetPath } catch { }
    Write-Output ($_.BaseName + "|" + $t)
  }
}`

func windowsStartMenuApps() []installedApp {
	out, err := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", windowsStartMenuScript).Output()
	if err != nil {
		return nil
	}
	var apps []installedApp
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 2)
		name := strings.TrimSpace(parts[0])
		if name == "" {
			continue
		}
		p := ""
		if len(parts) == 2 {
			p = strings.TrimSpace(parts[1])
		}
		apps = append(apps, installedApp{Name: name, Path: p})
	}
	return apps
}

func macApplications() []installedApp {
	home, _ := os.UserHomeDir()
	dirs := []string{"/Applications"}
	if home != "" {
		dirs = append(dirs, filepath.Join(home, "Applications"))
	}
	var apps []installedApp
	for _, d := range dirs {
		entries, err := os.ReadDir(d)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() && strings.HasSuffix(e.Name(), ".app") {
				apps = append(apps, installedApp{
					Name: strings.TrimSuffix(e.Name(), ".app"),
					Path: filepath.Join(d, e.Name()),
				})
			}
		}
	}
	return apps
}
