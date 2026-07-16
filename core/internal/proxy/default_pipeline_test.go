package proxy

import (
	"context"
	"testing"

	"centag/core/pkg/config"
)

func TestDefaultPipelineResolver(t *testing.T) {
	tests := []struct {
		name           string
		config         *config.Config
		expectedID     string
		expectedSource string
	}{
		{
			name:           "nil config",
			config:         nil,
			expectedID:     config.DefaultSystemPipelineID,
			expectedSource: "fallback",
		},
		{
			name: "config with default pipeline",
			config: &config.Config{
				Proxy: config.ProxyConfig{
					PipelineConfig: &config.PipelineConfig{
						DefaultPipeline: "audit-mode",
					},
				},
			},
			expectedID:     "audit-mode",
			expectedSource: "system-default",
		},
		{
			name: "config without default pipeline",
			config: &config.Config{
				Proxy: config.ProxyConfig{
					PipelineConfig: &config.PipelineConfig{},
				},
			},
			expectedID:     config.DefaultSystemPipelineID,
			expectedSource: "fallback",
		},
		{
			name: "config with default_mode only",
			config: &config.Config{
				Proxy: config.ProxyConfig{
					DefaultMode: "direct-backend",
				},
			},
			expectedID:     "direct-backend",
			expectedSource: "system-default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver := NewDefaultPipelineResolver(tt.config)
			id, source, err := resolver.Resolve(context.Background())

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if id != tt.expectedID {
				t.Errorf("expected pipeline ID %q, got %q", tt.expectedID, id)
			}
			if source != tt.expectedSource {
				t.Errorf("expected source %q, got %q", tt.expectedSource, source)
			}
		})
	}
}
