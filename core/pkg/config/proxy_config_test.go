package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestProxyConfigEffectiveDefaultPipeline(t *testing.T) {
	tests := []struct {
		name  string
		proxy ProxyConfig
		want  string
	}{
		{
			name: "pipeline config wins",
			proxy: ProxyConfig{
				DefaultMode:    "smart-scheduling",
				PipelineConfig: &PipelineConfig{DefaultPipeline: "direct-backend"},
			},
			want: "direct-backend",
		},
		{
			name:  "default mode fallback",
			proxy: ProxyConfig{DefaultMode: "direct-backend"},
			want:  "direct-backend",
		},
		{
			name:  "hard fallback is transparent",
			proxy: ProxyConfig{},
			want:  DefaultSystemPipelineID,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.proxy.EffectiveDefaultPipeline(); got != tt.want {
				t.Errorf("EffectiveDefaultPipeline() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDefaultProxyConfigUsesTransparentMode(t *testing.T) {
	t.Setenv("LLM_PROXY_DEFAULT_MODE", "unset-marker")
	if err := os.Unsetenv("LLM_PROXY_DEFAULT_MODE"); err != nil {
		t.Fatalf("Unsetenv: %v", err)
	}

	cfg := DefaultProxyConfig()
	if cfg.DefaultMode != DefaultSystemPipelineID {
		t.Fatalf("DefaultMode = %q, want %q", cfg.DefaultMode, DefaultSystemPipelineID)
	}
	if cfg.PipelineConfig == nil || cfg.PipelineConfig.DefaultPipeline != DefaultSystemPipelineID {
		t.Fatalf("PipelineConfig.DefaultPipeline = %#v, want %q", cfg.PipelineConfig, DefaultSystemPipelineID)
	}
}

func TestPersistProxyConfigWritesFileWhenDataDirSet(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CENTAG_DATA_DIR", dir)

	proxy := ProxyConfig{
		DefaultBackendID: "opencode-zen",
		DefaultModel:     "deepseek-v4-flash-free",
	}
	if err := PersistProxyConfig(context.Background(), proxy); err != nil {
		t.Fatalf("PersistProxyConfig: %v", err)
	}
	path := filepath.Join(dir, minimalProxyConfigFile)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected %s: %v", path, err)
	}
	loaded, err := LoadProxyConfigFromFile(dir, ProxyConfig{})
	if err != nil {
		t.Fatalf("LoadProxyConfigFromFile: %v", err)
	}
	if loaded.DefaultBackendID != "opencode-zen" || loaded.DefaultModel != "deepseek-v4-flash-free" {
		t.Fatalf("loaded = %+v", loaded)
	}
}
