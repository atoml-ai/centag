//go:build darwin

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

type darwinBackend struct{}

func newPlatform() Backend { return darwinBackend{} }

func (darwinBackend) Supported() (bool, string) {
	return true, "macOS networksetup + security"
}

func (darwinBackend) service() (string, error) {
	out, err := exec.Command("networksetup", "-listallnetworkservices").Output()
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "An asterisk") {
			continue
		}
		// Prefer Wi-Fi / Ethernet
		if strings.Contains(line, "Wi-Fi") || strings.Contains(line, "Ethernet") {
			return line, nil
		}
	}
	// fallback first real service
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "An asterisk") {
			return line, nil
		}
	}
	return "", fmt.Errorf("no network service found")
}

func (b darwinBackend) ReadProxy() (snapshot.ProxyState, error) {
	svc, err := b.service()
	if err != nil {
		return snapshot.ProxyState{}, err
	}
	out, err := exec.Command("networksetup", "-getautoproxyurl", svc).Output()
	if err != nil {
		return snapshot.ProxyState{Mode: "off"}, nil
	}
	text := string(out)
	state := snapshot.ProxyState{Mode: "off"}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "URL:") {
			state.PACURL = strings.TrimSpace(strings.TrimPrefix(line, "URL:"))
		}
		if strings.HasPrefix(line, "Enabled:") && strings.Contains(line, "Yes") {
			state.Mode = "pac"
		}
	}
	return state, nil
}

func (b darwinBackend) WritePAC(pacURL string) error {
	svc, err := b.service()
	if err != nil {
		return err
	}
	if err := exec.Command("networksetup", "-setautoproxyurl", svc, pacURL).Run(); err != nil {
		return fmt.Errorf("setautoproxyurl: %w", err)
	}
	if err := exec.Command("networksetup", "-setautoproxystate", svc, "on").Run(); err != nil {
		return fmt.Errorf("setautoproxystate: %w", err)
	}
	return nil
}

func (b darwinBackend) RestoreProxy(state snapshot.ProxyState) error {
	svc, err := b.service()
	if err != nil {
		return err
	}
	if state.Mode == "pac" && state.PACURL != "" {
		_ = exec.Command("networksetup", "-setautoproxyurl", svc, state.PACURL).Run()
		return exec.Command("networksetup", "-setautoproxystate", svc, "on").Run()
	}
	return exec.Command("networksetup", "-setautoproxystate", svc, "off").Run()
}

func fingerprintPEM(certPEM []byte) (string, error) {
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return "", fmt.Errorf("invalid PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(cert.Raw)
	return hex.EncodeToString(sum[:]), nil
}

func (darwinBackend) InstallCA(certPEM []byte) (string, error) {
	fp, err := fingerprintPEM(certPEM)
	if err != nil {
		return "", err
	}
	tmp := filepath.Join(os.TempDir(), "centag-ca-"+fp[:12]+".crt")
	if err := os.WriteFile(tmp, certPEM, 0o644); err != nil {
		return "", err
	}
	defer os.Remove(tmp)
	cmd := exec.Command("security", "add-trusted-cert", "-d", "-r", "trustRoot",
		"-k", "/Library/Keychains/System.keychain", tmp)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("security add-trusted-cert: %v (%s)", err, strings.TrimSpace(string(out)))
	}
	return fp, nil
}

func (darwinBackend) UninstallCA(fingerprint string) error {
	// Best-effort by common CN; fingerprint-selective delete needs cert hash tooling.
	cmd := exec.Command("security", "delete-certificate", "-c", "Centag CA",
		"/Library/Keychains/System.keychain")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("security delete-certificate: %v (%s) fingerprint=%s",
			err, strings.TrimSpace(string(out)), fingerprint)
	}
	return nil
}
