//go:build darwin

package main

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
)

// selectExecutable shows the native macOS application browser (Finder-style,
// with icons and search, rooted at /Applications) and returns the chosen .app's
// POSIX path ("" if cancelled).
func selectExecutable() (string, error) {
	script := `POSIX path of (choose file with prompt "选择要通过 Centag 代理启动的应用程序" of type {"com.apple.application-bundle"} default location (path to applications folder))`
	out, err := exec.Command("osascript", "-e", script).Output()
	if err != nil {
		// exit code 128 == user cancelled the dialog
		if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 128 {
			return "", nil
		}
		return "", fmt.Errorf("choose application: %w", err)
	}
	path := strings.TrimSpace(string(out))
	if path == "" {
		return "", nil
	}
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("selected path invalid: %w", err)
	}
	// A .app bundle is a directory, not an executable: resolve it to the real
	// Mach-O binary inside Contents/MacOS so `centag wrap run` can exec it.
	if exe, ok := resolveMacAppBundle(path); ok {
		return exe, nil
	}
	return path, nil
}

// launchWrapped starts commandLine in the background without any terminal
// window. Combined output goes to a timestamped log under ~/.centag/wrap/runs
// so failures stay diagnosable; the returned path is shown in the tray
// notification.
func launchWrapped(commandLine, label string) (string, error) {
	commandLine = strings.TrimSpace(commandLine)
	if commandLine == "" {
		return "", fmt.Errorf("empty command")
	}
	dir := filepath.Join(centagInstallRoot(), "wrap", "runs")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("log dir: %w", err)
	}
	logPath := filepath.Join(dir, fmt.Sprintf("%s-%s.log", time.Now().Format("20060102-150405"), sanitizeLogLabel(label)))
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return "", fmt.Errorf("create log: %w", err)
	}
	cmd := exec.Command("bash", "-c", commandLine)
	cmd.Stdout = f
	cmd.Stderr = f
	// Own process group: detached from the launcher's signal handling.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		_ = f.Close()
		return "", fmt.Errorf("start wrapped app: %w", err)
	}
	go func() {
		_ = cmd.Wait()
		_ = f.Close()
	}()
	return logPath, nil
}

// ensureCATrusted runs at startup: skip silently when the CA is already
// trusted (or headless); otherwise trigger the admin dialog once so a fresh
// install works out of the box.
func ensureCATrusted(cfg Config) {
	if cfg.Headless {
		return
	}
	caPath := locateCentagCA(cfg)
	if caPath == "" {
		return // sidecar not started yet; tray menu item covers manual trust
	}
	if _, err := exec.Command("security", "verify-cert", "-c", caPath, "-p", "ssl").CombinedOutput(); err == nil {
		return // already trusted
	}
	_ = exec.Command("security", "add-certificate", "-k", loginKeychain(), caPath).Run()
	trusted, err := trustCACert(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "centag-launcher: auto trust ca failed: %v\n", err)
		notifyUser("Centag", "CA 自动信任失败，可点托盘「信任 CA 证书」重试: "+err.Error())
		return
	}
	if trusted != "" {
		notifyUser("Centag", "Centag CA 已安装并信任，被代理应用可直接发起 HTTPS 请求")
	}
}

func loginKeychain() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "Library", "Keychains", "login.keychain-db")
}

// trustCACert installs the Centag CA into the System keychain as a trusted
// root via an admin-authorization dialog. Returns the CA path ("" if the user
// cancelled). Idempotent: re-running refreshes the trust settings, which also
// covers a regenerated CA after reinstall.
func trustCACert(cfg Config) (string, error) {
	caPath := locateCentagCA(cfg)
	if caPath == "" {
		return "", fmt.Errorf("未找到 CA 证书（请先启动 sidecar 并执行一次 wrap）")
	}
	script := fmt.Sprintf(
		`do shell script "security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain %s" with administrator privileges`,
		shellQuote(caPath),
	)
	if out, err := exec.Command("osascript", "-e", script).CombinedOutput(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 128 {
			return "", nil // cancelled
		}
		return "", fmt.Errorf("security add-trusted-cert: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return caPath, nil
}

// locateCentagCA finds the sidecar's root CA, preferring the lib install over
// the wrap download cache.
func locateCentagCA(cfg Config) string {
	candidates := []string{
		filepath.Join(cfg.DataDir, "certs", "ca.crt"),
		filepath.Join(centagInstallRoot(), "wrap", "ca.crt"),
	}
	for _, p := range candidates {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}

// resolveMacAppBundle maps a selected /Applications/Foo.app (a directory) to its
// actual executable. CFBundleExecutable is read with a sequential XML walk
// (index-based key/string pairing desyncs on real plists that mix
// <true/>/<integer> values), falling back to the bundle name and then to the
// first executable in Contents/MacOS (Electron apps ship their binary under a
// name like "Electron", e.g. WorkBuddy.app → Contents/MacOS/Electron).
func resolveMacAppBundle(path string) (string, bool) {
	path = strings.TrimRight(path, "/")
	if !strings.HasSuffix(path, ".app") {
		return "", false
	}
	macOSDir := filepath.Join(path, "Contents", "MacOS")
	binName := strings.TrimSuffix(filepath.Base(path), ".app")
	if data, err := os.ReadFile(filepath.Join(path, "Contents", "Info.plist")); err == nil {
		if exe, ok := plistTopLevelString(data, "CFBundleExecutable"); ok && exe != "" {
			binName = exe
		}
	}
	if st, err := os.Stat(filepath.Join(macOSDir, binName)); err == nil && !st.IsDir() {
		return filepath.Join(macOSDir, binName), true
	}
	if first, ok := firstExecutableIn(macOSDir); ok {
		return first, true
	}
	return "", false
}

// firstExecutableIn returns the first executable regular file in dir.
func firstExecutableIn(dir string) (string, bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", false
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for _, n := range names {
		p := filepath.Join(dir, n)
		if st, err := os.Stat(p); err == nil && st.Mode().Perm()&0o111 != 0 {
			return p, true
		}
	}
	return "", false
}

// plistTopLevelString returns the string value of key in the top-level <dict>
// of an XML property list. Keys are paired with the element that follows them,
// so values of other types (<true/>, <integer>, nested dicts) do not shift the
// mapping between keys and strings.
func plistTopLevelString(data []byte, want string) (string, bool) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	var key string
	haveKey := false
	dictDepth := 0
	topDict := -1
	for {
		tok, err := dec.Token()
		if err != nil {
			return "", false
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "dict":
				dictDepth++
				if haveKey {
					haveKey = false // the dict is the value of the pending key
				}
				if topDict == -1 {
					topDict = dictDepth
				}
			case "key":
				if dictDepth == topDict {
					var s string
					if dec.DecodeElement(&s, &t) == nil {
						key, haveKey = strings.TrimSpace(s), true
					}
				}
			case "string":
				if haveKey && dictDepth == topDict {
					var s string
					if err := dec.DecodeElement(&s, &t); err == nil && key == want {
						return strings.TrimSpace(s), true
					}
					haveKey = false
				}
			default:
				haveKey = false // <true/>, <integer>, <date>, <array> …
			}
		case xml.EndElement:
			if t.Name.Local == "dict" && dictDepth > 0 {
				if dictDepth == topDict {
					topDict = -2 // top-level dict closed; stop matching
				}
				dictDepth--
			}
		}
	}
}
