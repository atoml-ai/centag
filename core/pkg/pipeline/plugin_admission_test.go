package pipeline

import (
	"testing"

	"centag/core/pkg/config"
)

func TestAdmissionChecker_CheckPermissions(t *testing.T) {
	cfg := config.PluginAdmissionConfig{
		Enabled:          true,
		CheckPermissions: true,
	}
	checker := NewAdmissionChecker(cfg)

	tests := []struct {
		name       string
		descriptor NodePluginDescriptor
		wantPass   bool
		minScore   int
	}{
		{
			name: "minimal permissions",
			descriptor: NodePluginDescriptor{
				Name:           "test-plugin",
				Implementation: "test",
				Permissions:    []string{"memory.read"},
			},
			wantPass: true,
			minScore: 80,
		},
		{
			name: "no permissions",
			descriptor: NodePluginDescriptor{
				Name:           "test-plugin",
				Implementation: "test",
				Permissions:    []string{},
			},
			wantPass: true,
			minScore: 70,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := checker.CheckPermissions(tt.descriptor)
			if result.Passed != tt.wantPass {
				t.Errorf("CheckPermissions() Passed = %v, want %v", result.Passed, tt.wantPass)
			}
			if result.Score < tt.minScore {
				t.Errorf("CheckPermissions() Score = %v, want >= %v", result.Score, tt.minScore)
			}
		})
	}
}

func TestAdmissionChecker_CheckTimeout(t *testing.T) {
	cfg := config.PluginAdmissionConfig{
		Enabled:           true,
		CheckTimeout:      true,
		MaxTimeoutSeconds: 300,
		MinTimeoutSeconds: 5,
	}
	checker := NewAdmissionChecker(cfg)

	tests := []struct {
		name     string
		timeout  int
		wantPass bool
	}{
		{"valid timeout", 30, true},
		{"too short", 1, true},  // 太短产生警告但不失败
		{"too long", 400, false}, // 太长会失败
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := checker.CheckTimeout(tt.timeout)
			if result.Passed != tt.wantPass {
				t.Errorf("CheckTimeout() Passed = %v, want %v", result.Passed, tt.wantPass)
			}
		})
	}
}

func TestAdmissionChecker_CheckAll(t *testing.T) {
	cfg := config.PluginAdmissionConfig{
		Enabled:            true,
		CheckPermissions:   true,
		CheckTimeout:       true,
		CheckErrorHandling: true,
		CheckObservability: true,
		MaxTimeoutSeconds:  300,
		MinTimeoutSeconds:  5,
	}
	checker := NewAdmissionChecker(cfg)

	descriptor := NodePluginDescriptor{
		Name:           "test-plugin",
		Implementation: "test",
		Version:        "1.0.0",
		Permissions:    []string{"memory.read"},
		Tags:           []string{"test", "retry"},
	}

	result := checker.CheckAll(descriptor, 30, nil)

	if !result.Passed {
		t.Errorf("CheckAll() should pass, got Passed = %v", result.Passed)
	}

	if result.Score < 60 {
		t.Errorf("CheckAll() Score = %v, want >= 60", result.Score)
	}
}
