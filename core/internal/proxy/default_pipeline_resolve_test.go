package proxy

import (
	"context"
	"testing"

	"centag/core/pkg/config"
)

func TestDefaultPipelineResolverResolveProxyMode(t *testing.T) {
	tests := []struct {
		name           string
		config         *config.Config
		expectedMode   ProxyMode
		expectedSource string
	}{
		{
			name:           "nil config",
			config:         nil,
			expectedMode:   ProxyMode(config.DefaultSystemPipelineID),
			expectedSource: "fallback",
		},
		{
			name: "default_mode direct-backend",
			config: &config.Config{
				Proxy: config.ProxyConfig{DefaultMode: "direct-backend"},
			},
			expectedMode:   ModeDirectBackend,
			expectedSource: "system-default",
		},
		{
			name: "pipeline_config overrides default_mode",
			config: &config.Config{
				Proxy: config.ProxyConfig{
					DefaultMode: "smart-scheduling",
					PipelineConfig: &config.PipelineConfig{
						DefaultPipeline: "direct-backend",
					},
				},
			},
			expectedMode:   ModeDirectBackend,
			expectedSource: "system-default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver := NewDefaultPipelineResolver(tt.config)
			mode, source, err := resolver.ResolveProxyMode(context.Background(), "", nil, nil)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if mode != tt.expectedMode {
				t.Errorf("expected mode %q, got %q", tt.expectedMode, mode)
			}
			if source != tt.expectedSource {
				t.Errorf("expected source %q, got %q", tt.expectedSource, source)
			}
		})
	}
}