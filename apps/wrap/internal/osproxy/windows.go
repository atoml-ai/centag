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

	"centag/apps/wrap/internal/snapshot"
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
	manual := regQuery("ProxyServer")
	enableReg := regQuery("ProxyEnable")
	enabled := enableReg == "0x1" || enableReg == "1"

	if pac := regQuery("AutoConfigURL"); pac != "" {
		state.Mode = "pac"
		state.PACURL = pac
		// Centag's PAC takeover clears ProxyEnable, but ProxyServer keeps the
		// user's original manual proxy. Preserve it so upstream egress can be
		// recovered from the snapshot on disable/force re-enable.
		if manual != "" {
			state.HTTP = manual
			state.HTTPS = manual
		}
		return state, nil
	}
	if enabled {
		state.Mode = "manual"
		state.HTTP = manual
		state.HTTPS = manual
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
	// CurrentUser store (-user): no admin required, and it is what Schannel/
	// WinINET-based clients use for the logged-in user. Matches the desktop
	// launcher's ensureCATrusted.
	if out, err := exec.Command("certutil", "-user", "-addstore", "-f", "Root", tmp).CombinedOutput(); err != nil {
		return "", fmt.Errorf("certutil -user Root: %v (%s)", err, strings.TrimSpace(string(out)))
	}
	_ = exec.Command("certutil", "-user", "-addstore", "-f", "CA", tmp).Run()
	return fp, nil
}

// UninstallCA removes the CA by SHA-256 fingerprint instead of the "Centag CA"
// subject substring (P1-3). certutil's textual output is locale-dependent, so we
// enumerate the store with PowerShell and compare each certificate's SHA-256
// hash to the requested fingerprint. When the fingerprint is empty we fall back
// to the exact subject common name so a stale entry is still cleaned up.
func (windowsBackend) UninstallCA(fingerprint string) error {
	fingerprint = strings.ToLower(strings.TrimSpace(fingerprint))
	script := `param([string]$Sha256)
$ErrorActionPreference = 'SilentlyContinue'
foreach ($s in @('Root','CA')) {
  $path = "Cert:\CurrentUser\$s"
  if (-not (Test-Path $path)) { continue }
  Get-ChildItem $path | ForEach-Object {
    if ($Sha256.Length -gt 0) {
      $h = [System.Security.Cryptography.SHA256]::Create()
      $hex = ([System.BitConverter]::ToString($h.ComputeHash($_.RawData))).Replace('-','').ToLowerInvariant()
      if ($hex -eq $Sha256) { Remove-Item -Path $_.PSPath -Force }
    } elseif ($_.Subject -eq 'CN=Centag CA') {
      Remove-Item -Path $_.PSPath -Force
    }
  }
}
`
	out, err := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script, "-Sha256", fingerprint).CombinedOutput()
	if err != nil {
		return fmt.Errorf("powershell remove CA: %v (%s) fingerprint=%s", err, strings.TrimSpace(string(out)), fingerprint)
	}
	return nil
}
