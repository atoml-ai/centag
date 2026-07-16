package proxymode

import "testing"

func TestGetPipelineModel_RawAndTransparent(t *testing.T) {
	tests := []struct {
		mode ExecutionMode
		want string
	}{
		{ModeTransparentProxy, PipelineModelTransparentProxy},
		{ModeTransparentFast, PipelineModelTransparentFast},
		{ModeRawForward, PipelineModelRawForward},
		{ModeDirectBackend, PipelineModelDirectBackend},
	}
	for _, tt := range tests {
		t.Run(string(tt.mode), func(t *testing.T) {
			if got := GetPipelineModel(tt.mode); got != tt.want {
				t.Fatalf("GetPipelineModel(%s) = %q, want %q", tt.mode, got, tt.want)
			}
		})
	}
}
