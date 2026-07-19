package osproxy

import "centag/apps/proxyctl/internal/snapshot"

// Backend reads/writes OS proxy settings and CA trust.
type Backend interface {
	// ReadProxy captures current OS proxy into snapshot fields.
	ReadProxy() (snapshot.ProxyState, error)
	// WritePAC enables automatic proxy configuration URL.
	WritePAC(pacURL string) error
	// RestoreProxy restores a previous proxy state.
	RestoreProxy(state snapshot.ProxyState) error
	// InstallCA trusts the given PEM CA; returns sha256 fingerprint.
	InstallCA(certPEM []byte) (fingerprint string, err error)
	// UninstallCA removes CA by sha256 fingerprint.
	UninstallCA(fingerprint string) error
	// Supported reports whether full OS automation is available.
	Supported() (ok bool, detail string)
}

// New returns the platform backend.
func New() Backend {
	return newPlatform()
}
