package proxymode

import (
	"testing"
)

func TestGetPipelineModel(t *testing.T) {
	tests := []struct {
		name string
		mode ExecutionMode
		want string
	}{
		{"Fallback", ModeFallback, PipelineModelFallback},
		{"OptimizeMode", ModeOptimizeMode, PipelineModelOptimize},
		{"AuditMode", ModeAuditMode, PipelineModelAudit},
		{"Aggregator", ModeAggregator, PipelineModelAggregate},
		{"Translate", ModeTranslate, PipelineModelTranslate},
		{"SystemScheduling", ModeSystemScheduling, PipelineModelSmartScheduling},
		{"ModelMatching", ModeModelMatching, PipelineModelModelMatching},
		{"Router", ModeRouter, PipelineModelRouter},
		{"DirectBackend", ModeDirectBackend, PipelineModelDirectBackend},
		{"TransparentProxy", ModeTransparentProxy, PipelineModelTransparentProxy},
		{"TransparentFast", ModeTransparentFast, PipelineModelTransparentFast},
		{"Pipeline", ModePipeline, PipelineModelPipeline},
		{"CodingAgent", ModeCodingAgent, PipelineModelCodingAgent},
		{"UnknownMode", ExecutionMode("unknown"), ""},
		{"EmptyMode", ExecutionMode(""), ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetPipelineModel(tt.mode)
			if got != tt.want {
				t.Errorf("GetPipelineModel(%s) = %q, want %q", tt.mode, got, tt.want)
			}
		})
	}
}

func TestIsPipelineModel(t *testing.T) {
	tests := []struct {
		name  string
		model string
		want  bool
	}{
		{"ValidFallback", PipelineModelFallback, true},
		{"ValidOptimize", PipelineModelOptimize, true},
		{"ValidAudit", PipelineModelAudit, true},
		{"ValidAggregate", PipelineModelAggregate, true},
		{"ValidTranslate", PipelineModelTranslate, true},
		{"ValidSmartScheduling", PipelineModelSmartScheduling, true},
		{"ValidModelMatching", PipelineModelModelMatching, true},
		{"ValidRouter", PipelineModelRouter, true},
		{"ValidDirectBackend", PipelineModelDirectBackend, true},
		{"ValidTransparentProxy", PipelineModelTransparentProxy, true},
		{"ValidTransparentFast", PipelineModelTransparentFast, true},
		{"ValidPipeline", PipelineModelPipeline, true},
		{"ValidCodingAgent", PipelineModelCodingAgent, true},
		{"TooShort", "pipeline", false},
		{"EmptyString", "", false},
		{"NotPipeline", "gpt-4", false},
		{"ExactlyBoundaryMatch", "pipeline.x", true},
		{"MinValid", "pipeline.a", true},
		{"CentagPrefix", "centag/direct-backend", true},
		{"CentagPrefixShort", "centag/a", true},
		{"CentagPrefixOnly", "centag/", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsPipelineModel(tt.model)
			if got != tt.want {
				t.Errorf("IsPipelineModel(%q) = %v, want %v", tt.model, got, tt.want)
			}
		})
	}
}
