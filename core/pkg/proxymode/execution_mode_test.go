package proxymode

import (
	"testing"
)

func TestExecutionMode_String(t *testing.T) {
	tests := []struct {
		mode   ExecutionMode
		wantCN string
	}{
		{ModeDirectBackend, "指定后端"},
		{ModeTransparentProxy, "透明模式"},
		{ModeTransparentFast, "透明模式（快）"},
		{ModeFixedEgress, "跳板模式"},
		{ModeSystemScheduling, "系统调度"},
		{ModeModelMatching, "模型匹配"},
		{ModeIntentClassification, "意图分类"},
		{ModePipeline, "流水线编排"},
		{ModeFallback, "降级"},
		{ModeCustom, "自定义"},
		{ModeCodingAgent, "编程Agent"},
	}

	for _, tt := range tests {
		t.Run(string(tt.mode), func(t *testing.T) {
			if got := tt.mode.String(); got != tt.wantCN {
				t.Errorf("String() = %s, want %s", got, tt.wantCN)
			}
		})
	}
}

func TestExecutionMode_IsValid(t *testing.T) {
	tests := []struct {
		mode  ExecutionMode
		valid bool
	}{
		{ModeDirectBackend, true},
		{ModeTransparentProxy, true},
		{ModeTransparentFast, true},
		{ModeFixedEgress, true},
		{ModeSystemScheduling, true},
		{ModeModelMatching, true},
		{ModeIntentClassification, true},
		{ModePipeline, true},
		{ModeFallback, true},
		{ModeCustom, true},
		{ModeCodingAgent, true},
		{ExecutionMode("invalid"), false},
		{ExecutionMode(""), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.mode), func(t *testing.T) {
			if got := tt.mode.IsValid(); got != tt.valid {
				t.Errorf("IsValid() = %v, want %v for mode %s", got, tt.valid, tt.mode)
			}
		})
	}
}

func TestFromString(t *testing.T) {
	tests := []struct {
		input   string
		want    ExecutionMode
		wantErr bool
	}{
		// 直接模式名
		{"direct-backend", ModeDirectBackend, false},
		{"transparent-proxy", ModeTransparentProxy, false},
		{"transparent-fast", ModeTransparentFast, false},
		{"fixed-egress", ModeFixedEgress, false},
		{"system-scheduling", ModeSystemScheduling, false},
		{"model-matching", ModeModelMatching, false},
		{"intent-classification", ModeIntentClassification, false},
		{"pipeline", ModePipeline, false},

		// 关键字
		{"#d", ModeDirectBackend, false},
		{"#s", ModeSystemScheduling, false},
		{"#m", ModeModelMatching, false},
		{"#c", ModeIntentClassification, false},
		{"#p", ModePipeline, false},
		{"#t", ModeTransparentProxy, false},
		{"#tf", ModeTransparentFast, false},
		{"#j", ModeFixedEgress, false},
		{"#mem0", ModeMem0, false},

		// 类型名
		{"direct", ModeDirectBackend, false},
		{"schedule", ModeSystemScheduling, false},
		{"match", ModeModelMatching, false},
		{"classify", ModeIntentClassification, false},
		{"smart-scheduling", ModeSystemScheduling, false},
		{"model-matching", ModeModelMatching, false},
		{"transparent", ModeTransparentProxy, false},
		{"mem0", ModeMem0, false},
		{"mem0-memory", ModeMem0, false},
		{"coding-agent", ModeCodingAgent, false},
		{"#code", ModeCodingAgent, false},

		// 无效输入
		{"", "", true},
		{"invalid-mode", "", true},
		{"#x", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := FromString(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("FromString(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("FromString(%q) = %s, want %s", tt.input, got, tt.want)
			}
		})
	}
}

func TestExecutionMode_GetType(t *testing.T) {
	tests := []struct {
		mode     ExecutionMode
		wantType string
	}{
		{ModeDirectBackend, "direct"},
		{ModeTransparentProxy, "transparent"},
		{ModeTransparentFast, "transparent-fast"},
		{ModeFixedEgress, "fixed-egress"},
		{ModeSystemScheduling, "schedule"},
		{ModeModelMatching, "match"},
		{ModeIntentClassification, "classify"},
		{ModePipeline, "pipeline"},
		{ModeFallback, "fallback"},
		{ModeMem0, "mem0"},
		{ModeCustom, "custom"},
		{ModeCodingAgent, "coding-agent"},
	}

	for _, tt := range tests {
		t.Run(string(tt.mode), func(t *testing.T) {
			if got := tt.mode.GetType(); got != tt.wantType {
				t.Errorf("GetType() = %s, want %s", got, tt.wantType)
			}
		})
	}
}

func TestExecutionMode_GetKey(t *testing.T) {
	tests := []struct {
		mode    ExecutionMode
		wantKey string
	}{
		{ModeDirectBackend, "#d"},
		{ModeSystemScheduling, "#s"},
		{ModeModelMatching, "#m"},
		{ModeIntentClassification, "#c"},
		{ModePipeline, "#p"},
		{ModeTransparentProxy, "#t"},
		{ModeFallback, "#f"},
		{ModeMem0, "#mem0"},
		{ModeCustom, "#custom"},
	}

	for _, tt := range tests {
		t.Run(string(tt.mode), func(t *testing.T) {
			if got := tt.mode.GetKey(); got != tt.wantKey {
				t.Errorf("GetKey() = %s, want %s", got, tt.wantKey)
			}
		})
	}
}

func TestAllModes(t *testing.T) {
	modes := AllModes()
	if len(modes) != 19 {
		t.Errorf("AllModes() returned %d modes, want 19", len(modes))
	}
}

func TestCoreModes(t *testing.T) {
	modes := CoreModes()
	if len(modes) != 5 {
		t.Errorf("CoreModes() returned %d modes, want 5", len(modes))
	}
}
