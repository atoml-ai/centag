//go:build windows

package sysproxy

import (
	"strings"

	"golang.org/x/sys/windows/registry"
)

const inetSettingsKey = `Software\Microsoft\Windows\CurrentVersion\Internet Settings`

// Read returns the current WinINET (per-user) proxy configuration.
func Read() (State, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, inetSettingsKey, registry.QUERY_VALUE)
	if err != nil {
		return State{Mode: "off"}, err
	}
	defer k.Close()

	s := State{Mode: "off"}
	if v, _, err := k.GetStringValue("AutoConfigURL"); err == nil {
		s.PACURL = strings.TrimSpace(v)
	}
	if v, _, err := k.GetStringValue("ProxyServer"); err == nil {
		s.Manual = strings.TrimSpace(v)
	}
	s.Enabled = readProxyEnable(k)

	switch {
	case s.PACURL != "":
		s.Mode = "pac"
	case s.Enabled && s.Manual != "":
		s.Mode = "manual"
	}
	if s.Mode == "manual" {
		s.HTTP = s.Manual
		s.HTTPS = s.Manual
	}
	return s, nil
}

func readProxyEnable(k registry.Key) bool {
	if v, _, err := k.GetIntegerValue("ProxyEnable"); err == nil {
		return v == 1
	}
	if v, _, err := k.GetStringValue("ProxyEnable"); err == nil {
		return strings.TrimSpace(v) == "1"
	}
	return false
}
