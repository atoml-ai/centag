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
	// Check if CA is already installed and trusted in the System keychain.
	// Use verify-cert to avoid unnecessary admin privilege prompts.
	tmp := filepath.Join(os.TempDir(), "centag-ca-"+fp[:12]+".crt")
	if err := os.WriteFile(tmp, certPEM, 0o644); err != nil {
		return "", err
	}
	defer os.Remove(tmp)
	if out, err := exec.Command("security", "verify-cert", "-c", tmp, "-p", "ssl", "-k", "/Library/Keychains/System.keychain").CombinedOutput(); err == nil {
		_ = out
		// CA already installed and trusted
		return fp, nil
	}
	// CA not found or not trusted — install with admin privileges.
	script := fmt.Sprintf(
		`do shell script "security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain %s" with administrator privileges`,
		shellQuote(tmp),
	)
	if out, err := exec.Command("osascript", "-e", script).CombinedOutput(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 128 {
			return "", fmt.Errorf("user cancelled CA installation")
		}
		return "", fmt.Errorf("security add-trusted-cert: %v (%s)", err, strings.TrimSpace(string(out)))
	}
	return fp, nil
}

// shellQuote wraps s in single quotes for safe embedding in shell scripts.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func (darwinBackend) UninstallCA(fingerprint string) error {
	// Best-effort by common CN; fingerprint-selective delete needs cert hash tooling.
	// Use osascript to elevate privileges for deleting from System keychain.
	script := fmt.Sprintf(
		`do shell script "security delete-certificate -c 'Centag CA' /Library/Keychains/System.keychain" with administrator privileges`,
	)
	if out, err := exec.Command("osascript", "-e", script).CombinedOutput(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 128 {
			return nil // user cancelled, treat as best-effort success
		}
		return fmt.Errorf("security delete-certificate: %v (%s) fingerprint=%s",
			err, strings.TrimSpace(string(out)), fingerprint)
	}
	return nil
}
