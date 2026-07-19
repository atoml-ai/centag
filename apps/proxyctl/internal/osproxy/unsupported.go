//go:build !darwin && !windows && !linux

package osproxy

import (
	"fmt"

	"centag/apps/proxyctl/internal/snapshot"
)

type stub struct{}

func newPlatform() Backend { return stub{} }

func (stub) Supported() (bool, string) {
	return false, "unsupported OS"
}

func (stub) ReadProxy() (snapshot.ProxyState, error) {
	return snapshot.ProxyState{Mode: "off"}, nil
}

func (stub) WritePAC(string) error {
	return fmt.Errorf("OS proxy automation unsupported on this platform")
}

func (stub) RestoreProxy(snapshot.ProxyState) error { return nil }

func (stub) InstallCA([]byte) (string, error) {
	return "", fmt.Errorf("CA install unsupported on this platform")
}

func (stub) UninstallCA(string) error {
	return fmt.Errorf("CA uninstall unsupported on this platform")
}
