//go:build linux

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

type linuxBackend struct{}

func newPlatform() Backend { return linuxBackend{} }

func (linuxBackend) desktop() string {
	return strings.ToLower(os.Getenv("XDG_CURRENT_DESKTOP") + " " + os.Getenv("DESKTOP_SESSION"))
}

func (b linuxBackend) Supported() (bool, string) {
	d := b.desktop()
	if strings.Contains(d, "gnome") || strings.Contains(d, "unity") {
		if _, err := exec.LookPath("gsettings"); err == nil {
			return true, "GNOME gsettings"
		}
	}
	if strings.Contains(d, "kde") || strings.Contains(d, "plasma") {
		return false, "KDE detected: use env wrapper (auto write not enabled in M1)"
	}
	return false, "unsupported_desktop: use HTTPS_PROXY env wrapper"
}

func (b linuxBackend) ReadProxy() (snapshot.ProxyState, error) {
	ok, _ := b.Supported()
	if !ok {
		return snapshot.ProxyState{Mode: "off"}, nil
	}
	mode, _ := exec.Command("gsettings", "get", "org.gnome.system.proxy", "mode").Output()
	pac, _ := exec.Command("gsettings", "get", "org.gnome.system.proxy", "autoconfig-url").Output()
	state := snapshot.ProxyState{Mode: "off"}
	m := strings.Trim(strings.TrimSpace(string(mode)), "'\"")
	if m == "auto" {
		state.Mode = "pac"
		state.PACURL = strings.Trim(strings.TrimSpace(string(pac)), "'\"")
	}
	return state, nil
}

func (b linuxBackend) WritePAC(pacURL string) error {
	ok, detail := b.Supported()
	if !ok {
		return fmt.Errorf("%s", detail)
	}
	if err := exec.Command("gsettings", "set", "org.gnome.system.proxy", "autoconfig-url", pacURL).Run(); err != nil {
		return err
	}
	return exec.Command("gsettings", "set", "org.gnome.system.proxy", "mode", "auto").Run()
}

func (b linuxBackend) RestoreProxy(state snapshot.ProxyState) error {
	ok, _ := b.Supported()
	if !ok {
		return nil
	}
	if state.Mode == "pac" && state.PACURL != "" {
		_ = exec.Command("gsettings", "set", "org.gnome.system.proxy", "autoconfig-url", state.PACURL).Run()
		return exec.Command("gsettings", "set", "org.gnome.system.proxy", "mode", "auto").Run()
	}
	return exec.Command("gsettings", "set", "org.gnome.system.proxy", "mode", "none").Run()
}

func (linuxBackend) InstallCA(certPEM []byte) (string, error) {
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

	destDir := "/usr/local/share/ca-certificates"
	dest := filepath.Join(destDir, "centag-ca.crt")
	tmp := filepath.Join(os.TempDir(), "centag-ca.crt")
	if err := os.WriteFile(tmp, certPEM, 0o644); err != nil {
		return "", err
	}
	cmd := exec.Command("sudo", "cp", tmp, dest)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("install ca: %v (%s)", err, strings.TrimSpace(string(out)))
	}
	if out, err := exec.Command("sudo", "update-ca-certificates").CombinedOutput(); err != nil {
		return "", fmt.Errorf("update-ca-certificates: %v (%s)", err, strings.TrimSpace(string(out)))
	}
	return fp, nil
}

func (linuxBackend) UninstallCA(fingerprint string) error {
	dest := "/usr/local/share/ca-certificates/centag-ca.crt"
	_ = exec.Command("sudo", "rm", "-f", dest).Run()
	out, err := exec.Command("sudo", "update-ca-certificates", "--fresh").CombinedOutput()
	if err != nil {
		return fmt.Errorf("remove ca: %v (%s) fingerprint=%s", err, strings.TrimSpace(string(out)), fingerprint)
	}
	return nil
}
