package pipeline

import (
	"testing"

	"centag/core/pkg/backend"
)

func TestResolveBackendSupportedModel(t *testing.T) {
	cfg := &backend.BackendConfig{
		ID:         "opencode-go",
		Type:       "openai",
		ProbeModel: "deepseek-v4.1-flash",
		SupportedModels: []backend.ModelMapping{
			{RequestedModel: "deepseek-v4.1-flash", ActualModel: "deepseek-v4.1-flash"},
			{RequestedModel: "kimi-k3", ActualModel: "kimi-k3"},
		},
	}

	cases := []struct {
		name  string
		model string
		want  string
	}{
		{"supported exact unchanged", "deepseek-v4.1-flash", "deepseek-v4.1-flash"},
		{"supported second unchanged", "kimi-k3", "kimi-k3"},
		{"unsupported falls back to preferred", "key-model-A", "deepseek-v4.1-flash"},
		{"empty unchanged", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolveBackendSupportedModel(cfg, tc.model); got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}

	// 未探测到 supported_models 的后端保持原样，避免误改写。
	empty := &backend.BackendConfig{ID: "x", Type: "openai", ProbeModel: "m"}
	if got := resolveBackendSupportedModel(empty, "key-model-A"); got != "key-model-A" {
		t.Fatalf("empty supported list should pass through, got %q", got)
	}
}
