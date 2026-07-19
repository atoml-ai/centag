//go:build windows

package osproxy

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"centag/apps/proxyctl/internal/snapshot"
)

const inetKey = `HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings`

type windowsBackend struct{}

func newPlatform() Backend { return windowsBackend{} }

func (windowsBackend) Supported() (bool, string) {
	return true, "Windows WinINET (user) + certutil"
}

func regQuery(name string) string {
	out, err := exec.Command("reg", "query", inetKey, "/v", name).Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.Contains(line, name) {
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				return parts[len(parts)-1]
			}
		}
	}
	return ""
}

func (windowsBackend) ReadProxy() (snapshot.ProxyState, error) {
	state := snapshot.ProxyState{Mode: "off"}
	if pac := regQuery("AutoConfigURL"); pac != "" {
		state.Mode = "pac"
		state.PACURL = pac
	}
	if regQuery("ProxyEnable") == "0x1" || regQuery("ProxyEnable") == "1" {
		if state.Mode == "off" {
			state.Mode = "manual"
		}
		state.HTTP = regQuery("ProxyServer")
		state.HTTPS = state.HTTP
	}
	return state, nil
}

func (windowsBackend) WritePAC(pacURL string) error {
	if err := exec.Command("reg", "add", inetKey, "/v", "AutoConfigURL", "/t", "REG_SZ", "/d", pacURL, "/f").Run(); err != nil {
		return fmt.Errorf("set AutoConfigURL: %w", err)
	}
	_ = exec.Command("reg", "add", inetKey, "/v", "ProxyEnable", "/t", "REG_DWORD", "/d", "0", "/f").Run()
	return nil
}

func (windowsBackend) RestoreProxy(state snapshot.ProxyState) error {
	switch state.Mode {
	case "pac":
		_ = exec.Command("reg", "add", inetKey, "/v", "AutoConfigURL", "/t", "REG_SZ", "/d", state.PACURL, "/f").Run()
		_ = exec.Command("reg", "add", inetKey, "/v", "ProxyEnable", "/t", "REG_DWORD", "/d", "0", "/f").Run()
	case "manual":
		_ = exec.Command("reg", "add", inetKey, "/v", "AutoConfigURL", "/t", "REG_SZ", "/d", "", "/f").Run()
		_ = exec.Command("reg", "add", inetKey, "/v", "ProxyEnable", "/t", "REG_DWORD", "/d", "1", "/f").Run()
		if state.HTTP != "" {
			_ = exec.Command("reg", "add", inetKey, "/v", "ProxyServer", "/t", "REG_SZ", "/d", state.HTTP, "/f").Run()
		}
	default:
		_ = exec.Command("reg", "add", inetKey, "/v", "AutoConfigURL", "/t", "REG_SZ", "/d", "", "/f").Run()
		_ = exec.Command("reg", "add", inetKey, "/v", "ProxyEnable", "/t", "REG_DWORD", "/d", "0", "/f").Run()
	}
	return nil
}

func (windowsBackend) InstallCA(certPEM []byte) (string, error) {
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return "", fmt.Errorf("invalid PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(cert.Raw)
	fp := hex.EncodeToString(sum[:])
	tmp := filepath.Join(os.TempDir(), "centag-ca-"+fp[:12]+".crt")
	if err := os.WriteFile(tmp, certPEM, 0o644); err != nil {
		return "", err
	}
	defer os.Remove(tmp)
	cmd := exec.Command("certutil", "-addstore", "-f", "Root", tmp)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("certutil: %v (%s)", err, strings.TrimSpace(string(out)))
	}
	return fp, nil
}

func (windowsBackend) UninstallCA(fingerprint string) error {
	cmd := exec.Command("certutil", "-delstore", "Root", "Centag CA")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("certutil -delstore: %v (%s) fingerprint=%s",
			err, strings.TrimSpace(string(out)), fingerprint)
	}
	return nil
}
