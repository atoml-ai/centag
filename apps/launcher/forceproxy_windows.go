//go:build windows

package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const defaultMITMProxy = "127.0.0.1:8081"

// mitmAddress resolves the Centag MITM proxy address (host:port) from the side
// wrap snapshot (%LOCALAPPDATA%\Centag\proxy-snapshot.json); falls back to the
// default loopback port used by the personal edition.
func mitmAddress() string {
	local := strings.TrimSpace(os.Getenv("LOCALAPPDATA"))
	if local == "" {
		return defaultMITMProxy
	}
	data, err := os.ReadFile(filepath.Join(local, "Centag", "proxy-snapshot.json"))
	if err != nil {
		return defaultMITMProxy
	}
	var snap struct {
		Centag struct {
			MITMProxy string `json:"mitm_proxy"`
		} `json:"centag"`
	}
	if err := json.Unmarshal(data, &snap); err != nil {
		return defaultMITMProxy
	}
	if s := strings.TrimSpace(snap.Centag.MITMProxy); s != "" {
		return s
	}
	return defaultMITMProxy
}

// chromiumProxyArgs builds the Chromium command-line switches that bind the
// app's proxy resolution to Centag — overrides both the OS proxy and any VPN
// client that rewrites WestINET system settings.
func chromiumProxyArgs(mitm string) []string {
	return []string{
		"--proxy-server=http://" + mitm,
		"--proxy-bypass-list=<local>",
		"--no-first-run",
	}
}

// proxyEnv builds per-process env for Electron/Node children that follow
// environment proxy variables (HTTPS_PROXY etc.) rather than Chromium flags.
func proxyEnv(mitm string) []string {
	p := "http://" + mitm
	return []string{
		"HTTP_PROXY=" + p,
		"HTTPS_PROXY=" + p,
		"NO_PROXY=localhost,127.0.0.1,::1,<local>",
	}
}

// openAppChromium launches a Chromium/Electron desktop app with the Centag
// force-proxy switches. MSIX store apps are resolved to their inner exe via
// AppxManifest; anything we cannot resolve falls back to the process-proxy
// launch path (explorer/cmd start), which still covers MITM + whitelist if the
// OS proxy is held by Centag.
func openAppChromium(app catalogApp) error {
	path := strings.TrimSpace(app.Path)
	if path == "" {
		return fmt.Errorf("application path not detected")
	}
	mitm := mitmAddress()
	args := chromiumProxyArgs(mitm)
	env := append(os.Environ(), proxyEnv(mitm)...)

	if isAUMID(path) {
		if exe, err := msixExeFor(path); err == nil && exe != "" {
			if _, statErr := os.Stat(exe); statErr == nil {
				return startGUI(exe, args, env)
			}
		}
		// We can only launch this store app via explorer activation, which
		// accepts no Chromium args. Signal the caller instead of launching an
		// unproxied app.
		return errNoPerProcessProxy
	}
	if st, err := os.Stat(path); err == nil && !st.IsDir() {
		return startGUI(path, args, env)
	}
	return errNoPerProcessProxy
}

func startGUI(exe string, args, env []string) error {
	cmd := exec.Command(exe, args...)
	cmd.Env = env
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start %s: %w", filepath.Base(exe), err)
	}
	return nil
}

// msixPackageLocation resolves the install location of a package via
// PowerShell Get-AppxPackage (WindowsApps is ACL-locked to admin list access).
var msixPackageLocation = func(name string) (string, error) {
	out, err := exec.Command("powershell", "-NoProfile", "-WindowStyle", "Hidden",
		"-Command", fmt.Sprintf("(Get-AppxPackage -Name %s).InstallLocation", name)).Output()
	if err != nil {
		return "", fmt.Errorf("get-appxpackage %s: %w", name, err)
	}
	loc := strings.TrimSpace(string(out))
	if loc == "" {
		return "", fmt.Errorf("package %s not found", name)
	}
	return loc, nil
}

// msixExeFor maps an AUMID (`Family!AppId`) to the inner executable declared by
// the package manifest.
func msixExeFor(aumid string) (string, error) {
	pfn := aumid
	if i := strings.IndexByte(aumid, '!'); i > 0 {
		pfn = aumid[:i]
	}
	name := pfn
	if i := strings.LastIndex(pfn, "_"); i > 0 {
		name = pfn[:i]
	}
	loc, err := msixPackageLocation(name)
	if err != nil {
		return "", err
	}
	manData, err := os.ReadFile(filepath.Join(loc, "AppxManifest.xml"))
	if err != nil {
		return "", fmt.Errorf("read manifest: %w", err)
	}
	rel, err := parseAppxManifestExe(manData)
	if err != nil {
		return "", err
	}
	return filepath.Join(loc, filepath.FromSlash(rel)), nil
}

// parseAppxManifestExe extracts the first Application.Executable attribute.
func parseAppxManifestExe(xmlData []byte) (string, error) {
	dec := xml.NewDecoder(strings.NewReader(string(xmlData)))
	dec.Strict = false
	dec.AutoClose = xml.HTMLAutoClose
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return "", fmt.Errorf("no Application.Executable in manifest")
		}
		if err != nil {
			return "", fmt.Errorf("parse manifest: %w", err)
		}
		start, ok := tok.(xml.StartElement)
		if !ok || !strings.EqualFold(start.Name.Local, "Application") {
			continue
		}
		for _, attr := range start.Attr {
			if strings.EqualFold(attr.Name.Local, "Executable") &&
				strings.TrimSpace(attr.Value) != "" {
				return strings.TrimSpace(attr.Value), nil
			}
		}
	}
}
