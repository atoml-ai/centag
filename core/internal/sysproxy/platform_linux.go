//go:build linux

package sysproxy

import (
	"os"
	"os/exec"
	"strings"
)

// Read returns the proxy configuration advertised via environment variables
// (the convention CLI agents follow) with a GNOME gsettings fallback.
func Read() (State, error) {
	s := State{Mode: "off"}
	for _, key := range []string{"https_proxy", "HTTPS_PROXY", "all_proxy", "ALL_PROXY", "http_proxy", "HTTP_PROXY"} {
		v := strings.TrimSpace(os.Getenv(key))
		if v == "" {
			continue
		}
		s.Manual = v
		s.HTTP = v
		s.HTTPS = v
		s.Enabled = true
		s.Mode = "manual"
		break
	}
	if s.Mode == "off" {
		if v := gsettingsProxy(); v != "" {
			s.Manual = v
			s.HTTP = v
			s.HTTPS = v
			s.Enabled = true
			s.Mode = "manual"
		}
	}
	return s, nil
}

func gsettingsProxy() string {
	if _, err := exec.LookPath("gsettings"); err != nil {
		return ""
	}
	modeOut, err := exec.Command("gsettings", "get", "org.gnome.system.proxy", "mode").Output()
	if err != nil || !strings.Contains(string(modeOut), "manual") {
		return ""
	}
	hostOut, err := exec.Command("gsettings", "get", "org.gnome.system.proxy.http", "host").Output()
	if err != nil {
		return ""
	}
	portOut, err := exec.Command("gsettings", "get", "org.gnome.system.proxy.http", "port").Output()
	if err != nil {
		return ""
	}
	host := strings.Trim(strings.TrimSpace(string(hostOut)), "'\"")
	port := strings.TrimSpace(string(portOut))
	if host == "" || port == "0" {
		return ""
	}
	return "http://" + host + ":" + port
}
