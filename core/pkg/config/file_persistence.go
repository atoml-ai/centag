package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// minimalProxyConfigFile 是极简版保存默认后端/模型配置的相对路径。
const minimalProxyConfigFile = "proxy-config.yaml"

// ProxyConfigFile is the on-disk representation used by the minimal edition.
type ProxyConfigFile struct {
	DefaultBackendID string `yaml:"default_backend_id" json:"default_backend_id"`
	DefaultModel     string `yaml:"default_model" json:"default_model"`
}

// ResolveDataDir returns the data directory used for file-based persistence.
// It respects the CENTAG_DATA_DIR environment variable and falls back to common
// paths relative to the executable.
func ResolveDataDir() string {
	if envDir := os.Getenv("CENTAG_DATA_DIR"); envDir != "" {
		return envDir
	}

	execPath, err := os.Executable()
	if err != nil {
		return ""
	}
	execDir := filepath.Dir(execPath)

	home, _ := os.UserHomeDir()
	candidates := []string{
		filepath.Join(execDir, "data"),
		filepath.Join(execDir, "..", "data"),
		filepath.Join(execDir, "..", "bin", "server", "data"),
		"./data",
		"../data",
		"./bin/server/data",
		"../bin/server/data",
	}
	if home != "" {
		candidates = append(candidates,
			filepath.Join(home, ".centag", "lib", "personal", "data"),
			filepath.Join(home, ".centag", "lib", "minimal", "data"),
		)
	}
	for _, dir := range candidates {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			absDir, err := filepath.Abs(dir)
			if err == nil {
				return absDir
			}
			return dir
		}
	}
	return ""
}

// LoadProxyConfigFromFile loads the persisted proxy config for the minimal edition.
// If the file does not exist, it returns the provided default config and no error.
func LoadProxyConfigFromFile(dataDir string, defaultCfg ProxyConfig) (ProxyConfig, error) {
	path := filepath.Join(dataDir, minimalProxyConfigFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return defaultCfg, nil
		}
		return defaultCfg, err
	}

	var fileCfg ProxyConfigFile
	if err := yaml.Unmarshal(data, &fileCfg); err != nil {
		return defaultCfg, fmt.Errorf("failed to parse proxy config %s: %w", path, err)
	}

	cfg := defaultCfg
	if fileCfg.DefaultBackendID != "" {
		cfg.DefaultBackendID = fileCfg.DefaultBackendID
	}
	if fileCfg.DefaultModel != "" {
		cfg.DefaultModel = fileCfg.DefaultModel
	}
	return cfg, nil
}

// SaveProxyConfigToFile persists the proxy config for the minimal edition.
func SaveProxyConfigToFile(dataDir string, cfg ProxyConfig) error {
	if dataDir == "" {
		return fmt.Errorf("data directory is empty")
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return fmt.Errorf("failed to create data dir %s: %w", dataDir, err)
	}

	fileCfg := ProxyConfigFile{
		DefaultBackendID: cfg.DefaultBackendID,
		DefaultModel:     cfg.DefaultModel,
	}
	data, err := yaml.Marshal(fileCfg)
	if err != nil {
		return fmt.Errorf("failed to marshal proxy config: %w", err)
	}

	path := filepath.Join(dataDir, minimalProxyConfigFile)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("failed to write proxy config %s: %w", path, err)
	}
	return nil
}
