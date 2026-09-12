// Package appenv loads the unified runtime configuration file
// (~/.centag/centag.conf) into process environment variables.
//
// Loading strategy (priority from high to low):
//  1. Real environment variables (never overwritten)
//  2. ~/.centag/centag.conf  (single unified config file; dev and deploy)
//  3. Built-in code defaults
//
// Legacy config locations (config/secrets/.env, .env.middleware,
// deploy/stack/.env) are deprecated: they are never loaded, but their
// presence is reported as a warning so users know to migrate.
package appenv

import (
	"os"
	"path/filepath"
	"strings"
)

// confFileName is the unified configuration file name.
const confFileName = "centag.conf"

// legacyRelPaths are deprecated config locations (relative to the working
// directory, i.e. the repo root in dev environments). They are never loaded;
// only detected so a migration warning can be emitted.
var legacyRelPaths = []string{
	"config/secrets/.env",
	"config/secrets/.env.middleware",
	"deploy/stack/.env",
}

// legacyWarnings collects deprecation warnings produced during Load so the
// caller can emit them once the logger is ready.
var legacyWarnings []string

// Home returns the centag home directory ($CENTAG_HOME or ~/.centag).
func Home() string {
	if h := os.Getenv("CENTAG_HOME"); h != "" {
		return h
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ".centag"
	}
	return filepath.Join(home, ".centag")
}

// ConfPath returns the unified config file path.
func ConfPath() string {
	return filepath.Join(Home(), confFileName)
}

// LegacyWarnings returns deprecation warnings detected during Load.
func LegacyWarnings() []string {
	return legacyWarnings
}

// Load reads the unified config file and sets every variable that is not
// already present in the environment (real env vars always win).
// It is safe to call multiple times; subsequent calls are no-ops.
// Missing file is not an error (built-in defaults apply).
func Load() error {
	if loaded {
		return nil
	}
	loaded = true

	detectLegacy()

	path := ConfPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, kv := range parse(string(data)) {
		if _, exists := os.LookupEnv(kv.key); !exists {
			_ = os.Setenv(kv.key, kv.value)
		}
	}
	return nil
}

var loaded bool

// pair is one parsed KEY=VALUE entry.
type pair struct {
	key   string
	value string
}

// parse extracts KEY=VALUE pairs. Supported syntax:
//   - comments (# ...) and blank lines are skipped
//   - optional "export " prefix
//   - optional surrounding single/double quotes on the value
//   - inline values are taken literally (no $ expansion)
func parse(content string) []pair {
	var out []pair
	for _, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		eq := strings.Index(line, "=")
		if eq <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		value := strings.TrimSpace(line[eq+1:])
		if idx := strings.Index(value, " #"); idx >= 0 && !strings.HasPrefix(value, "\"") && !strings.HasPrefix(value, "'") {
			value = strings.TrimSpace(value[:idx])
		}
		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}
		if key == "" {
			continue
		}
		out = append(out, pair{key: key, value: value})
	}
	return out
}

// detectLegacy records warnings for deprecated config files that still exist.
func detectLegacy() {
	for _, rel := range legacyRelPaths {
		if _, err := os.Stat(rel); err == nil {
			legacyWarnings = append(legacyWarnings,
				"已废弃配置文件仍存在: "+rel+"（不再加载，请迁移到 "+ConfPath()+"）")
		}
	}
}
