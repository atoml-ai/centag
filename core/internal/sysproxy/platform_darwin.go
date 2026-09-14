//go:build darwin

package sysproxy

import (
	"os/exec"
	"strconv"
	"strings"
)

// Read returns the current macOS proxy configuration via `scutil --proxy`.
func Read() (State, error) {
	out, err := exec.Command("scutil", "--proxy").Output()
	if err != nil {
		return State{Mode: "off"}, err
	}
	vals := parseScutil(string(out))
	s := State{Mode: "off"}

	if vals["ProxyAutoConfigEnable"] == "1" {
		s.PACURL = vals["ProxyAutoConfigURLString"]
		s.Mode = "pac"
	}
	if vals["HTTPEnable"] == "1" && vals["HTTPProxy"] != "" {
		s.HTTP = joinHostPort(vals["HTTPProxy"], vals["HTTPPort"])
		s.Manual = s.HTTP
		s.Enabled = true
		if s.Mode == "off" {
			s.Mode = "manual"
		}
	}
	if vals["HTTPSEnable"] == "1" && vals["HTTPSProxy"] != "" {
		s.HTTPS = joinHostPort(vals["HTTPSProxy"], vals["HTTPSPort"])
		s.Manual = s.HTTPS
		s.Enabled = true
		if s.Mode == "off" {
			s.Mode = "manual"
		}
	}
	return s, nil
}

func parseScutil(out string) map[string]string {
	vals := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		vals[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
	}
	return vals
}

func joinHostPort(host, port string) string {
	host = strings.TrimSpace(host)
	port = strings.TrimSpace(port)
	if host == "" {
		return ""
	}
	if port == "" {
		return host
	}
	if _, err := strconv.Atoi(port); err != nil {
		return host
	}
	return host + ":" + port
}
