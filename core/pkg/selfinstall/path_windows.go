//go:build windows

package selfinstall

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const envPathValue = "Path"

// appendBinDirToPath persists binDir in two places so all shells pick it up:
//  1. Windows registry (HKCU\Environment\Path) — for CMD / PowerShell
//  2. Bash rc file (~/.bash_profile) — for Git Bash / MSYS2 / MinGW
//
// Administrator privileges are not required.
func appendBinDirToPath(root, binDir string) (pathResult, error) {
	// 1. Windows registry (CMD / PowerShell).
	val, expand, exists, err := readUserPath()
	if err != nil {
		return pathResult{}, err
	}
	regChanged := false
	if !exists || !pathListContains(val, binDir) {
		newVal := val
		if newVal != "" && !strings.HasSuffix(newVal, ";") {
			newVal += ";"
		}
		newVal += binDir
		if !exists {
			expand = true // conventional type for user Path
		}
		if err := writeUserPath(newVal, expand); err != nil {
			return pathResult{}, err
		}
		broadcastEnvironmentChange()
		regChanged = true
	}

	// 2. Bash rc file (Git Bash / MSYS2).
	bashRes, bashErr := AppendBashRcPath(root, binDir)

	// Merge results.
	detail := `HKCU\Environment\Path`
	if regChanged {
		detail += " + bash rc"
	} else if bashRes.changed {
		detail = bashRes.detail
	} else {
		detail = `HKCU\Environment\Path` + " (already configured)"
	}
	if bashErr != nil {
		return pathResult{}, bashErr
	}
	return pathResult{changed: regChanged || bashRes.changed, detail: detail}, nil
}

// removeBinDirFromPath strips binDir from the Windows registry PATH and from
// bash rc files, then broadcasts WM_SETTINGCHANGE.
func removeBinDirFromPath(root, binDir string) (pathResult, error) {
	// 1. Windows registry (CMD / PowerShell).
	val, expand, exists, err := readUserPath()
	if err != nil {
		return pathResult{}, err
	}
	regChanged := false
	if exists && pathListContains(val, binDir) {
		kept := make([]string, 0, 16)
		for _, entry := range splitPathList(val) {
			if sameWinPath(entry, binDir) {
				continue
			}
			kept = append(kept, entry)
		}
		if err := writeUserPath(strings.Join(kept, ";"), expand); err != nil {
			return pathResult{}, err
		}
		broadcastEnvironmentChange()
		regChanged = true
	}

	// 2. Bash rc file (Git Bash / MSYS2).
	bashRes, _ := RemoveBashRcPath(root, binDir)

	detail := `HKCU\Environment\Path`
	if regChanged && bashRes.changed {
		detail += " + bash rc"
	} else if bashRes.changed {
		detail = bashRes.detail
	}
	return pathResult{changed: regChanged || bashRes.changed, detail: detail}, nil
}

// readUserPath returns the raw (unexpanded) user Path value and its kind.
func readUserPath() (value string, expand bool, exists bool, err error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Environment`, registry.QUERY_VALUE)
	if err != nil {
		if os.IsNotExist(err) {
			return "", true, false, nil
		}
		return "", false, false, fmt.Errorf("open HKCU\\Environment: %w", err)
	}
	defer k.Close()
	s, typ, err := k.GetStringValue(envPathValue)
	switch {
	case err == nil:
		return s, typ == registry.EXPAND_SZ, true, nil
	case errors.Is(err, registry.ErrNotExist):
		return "", true, false, nil
	default:
		return "", false, false, err
	}
}

func writeUserPath(value string, expand bool) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, `Environment`, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open HKCU\\Environment: %w", err)
	}
	defer k.Close()
	if expand {
		return k.SetExpandStringValue(envPathValue, value)
	}
	return k.SetStringValue(envPathValue, value)
}

func splitPathList(list string) []string {
	entries := strings.Split(list, ";")
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if strings.TrimSpace(e) != "" {
			out = append(out, e)
		}
	}
	return out
}

func pathListContains(list, dir string) bool {
	for _, entry := range splitPathList(list) {
		if sameWinPath(entry, dir) {
			return true
		}
	}
	return false
}

// sameWinPath compares two windows paths case-insensitively, ignoring a
// trailing separator.
func sameWinPath(a, b string) bool {
	norm := func(p string) string {
		p = strings.TrimSpace(p)
		p = filepath.Clean(p)
		p = strings.TrimSuffix(p, string(os.PathSeparator))
		return strings.ToLower(p)
	}
	return norm(a) == norm(b)
}

var (
	user32DLL        = windows.NewLazySystemDLL("user32.dll")
	procSendMessageW = user32DLL.NewProc("SendMessageTimeoutW")
)

// broadcastEnvironmentChange notifies running processes that the user
// environment changed (WM_SETTINGCHANGE, "Environment").
func broadcastEnvironmentChange() {
	const (
		hwndBroadcast   = 0xFFFF
		wmSettingChange = 0x001A
		smtoAbortIfHung = 0x0002
		timeoutMs       = 1000
	)
	envName, err := syscall.UTF16FromString("Environment")
	if err != nil {
		return
	}
	_ = procSendMessageW.Find()
	procSendMessageW.Call(
		uintptr(hwndBroadcast),
		uintptr(wmSettingChange),
		0,
		uintptr(unsafe.Pointer(&envName[0])),
		uintptr(smtoAbortIfHung),
		uintptr(timeoutMs),
		0,
	)
}
