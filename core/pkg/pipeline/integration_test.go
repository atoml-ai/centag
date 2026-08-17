package pipeline

import (
	"testing"
)

func TestIntegration_AllLegacyPipelinesConverted(t *testing.T) {
	compat := NewConfigCompatLayer()

	// 测试所有旧版流水线是否正确转换
	legacyPipelines := []struct {
		name           string
		oldID          string
		expectedNewID  string
		expectedType   string
	}{
		{
			name:          "transparent-proxy",
			oldID:         "transparent-proxy",
			expectedNewID: "transparent",
			expectedType:  "transparent",
		},
		{
			name:          "direct-backend",
			oldID:         "direct-backend",
			expectedNewID: "transparent",
			expectedType:  "transparent",
		},
		{
			name:          "fixed-egress",
			oldID:         "fixed-egress",
			expectedNewID: "transparent",
			expectedType:  "transparent",
		},
		{
			name:          "router-mode",
			oldID:         "router-mode",
			expectedNewID: "router-pipeline",
			expectedType:  "router",
		},
		{
			name:          "agent-skill-router",
			oldID:         "agent-skill-router",
			expectedNewID: "centag-ops-router",
			expectedType:  "router",
		},
		{
			name:          "cache-hit",
			oldID:         "cache-hit",
			expectedNewID: "cache-pipeline",
			expectedType:  "cache",
		},
		{
			name:          "cache-mode",
			oldID:         "cache-mode",
			expectedNewID: "cache-pipeline",
			expectedType:  "cache",
		},
		{
			name:          "18-rag-mode",
			oldID:         "18-rag-mode",
			expectedNewID: "cache-pipeline",
			expectedType:  "cache",
		},
	}

	for _, tt := range legacyPipelines {
		t.Run(tt.name, func(t *testing.T) {
			config := map[string]interface{}{
				"id": tt.oldID,
			}

			// 测试ID转换
			newID := compat.GetActualPipelineID(tt.oldID)
			if newID != tt.expectedNewID {
				t.Errorf("GetActualPipelineID(%s) = %v, want %v", tt.oldID, newID, tt.expectedNewID)
			}

			// 测试流水线类型
			pipelineType := compat.GetPipelineType(config)
			if pipelineType != tt.expectedType {
				t.Errorf("GetPipelineType(%s) = %v, want %v", tt.oldID, pipelineType, tt.expectedType)
			}

			// 测试是否为旧版流水线
			if !compat.IsLegacyPipeline(config) {
				t.Errorf("IsLegacyPipeline(%s) should return true", tt.oldID)
			}
		})
	}
}

func TestIntegration_NewPipelinesNotLegacy(t *testing.T) {
	compat := NewConfigCompatLayer()

	// 测试新版流水线是否不被识别为旧版
	newPipelines := []string{
		"transparent",
		"router-pipeline",
		"centag-ops-router",
		"cache-pipeline",
	}

	for _, pipelineID := range newPipelines {
		t.Run(pipelineID, func(t *testing.T) {
			config := map[string]interface{}{
				"id": pipelineID,
			}

			if compat.IsLegacyPipeline(config) {
				t.Errorf("IsLegacyPipeline(%s) should return false", pipelineID)
			}
		})
	}
}

func TestIntegration_ShortcutCodeMapping(t *testing.T) {
	compat := NewConfigCompatLayer()

	// 测试快捷方式映射
	shortcuts := []struct {
		input    string
		expected string
	}{
		{"#skill", "#ops"},
		{"#t", "#t"},
		{"#d", "#d"},
		{"#j", "#j"},
		{"#r", "#r"},
		{"#ops", "#ops"},
	}

	for _, tt := range shortcuts {
		t.Run(tt.input, func(t *testing.T) {
			result := compat.GetActualShortcutCode(tt.input)
			if result != tt.expected {
				t.Errorf("GetActualShortcutCode(%s) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIntegration_PipelineTypeDetection(t *testing.T) {
	compat := NewConfigCompatLayer()

	// 测试流水线类型检测
	pipelines := []struct {
		id           string
		expectedType string
	}{
		{"transparent-proxy", "transparent"},
		{"direct-backend", "transparent"},
		{"fixed-egress", "transparent"},
		{"transparent", "transparent"},
		{"transparent-passthrough", "transparent"},
		{"router-mode", "router"},
		{"agent-skill-router", "router"},
		{"centag-ops-router", "router"},
		{"router-pipeline", "router"},
		{"cache-hit", "cache"},
		{"cache-mode", "cache"},
		{"18-rag-mode", "cache"},
		{"cache-pipeline", "cache"},
		{"unknown-pipeline", "unknown"},
	}

	for _, tt := range pipelines {
		t.Run(tt.id, func(t *testing.T) {
			config := map[string]interface{}{
				"id": tt.id,
			}

			result := compat.GetPipelineType(config)
			if result != tt.expectedType {
				t.Errorf("GetPipelineType(%s) = %v, want %v", tt.id, result, tt.expectedType)
			}
		})
	}
}

func TestIntegration_ConfigConversionPreservesMetadata(t *testing.T) {
	compat := NewConfigCompatLayer()

	// 测试配置转换是否保留metadata
	oldConfig := map[string]interface{}{
		"id":            "transparent-proxy",
		"shortcut_code": "#t",
		"metadata": map[string]interface{}{
			"custom_field": "custom_value",
		},
		"nodes": []interface{}{
			map[string]interface{}{
				"id":   "forward",
				"type": "transparent_forward",
				"config": map[string]interface{}{
					"custom_config": map[string]interface{}{
						"route_policy": "match_model",
					},
				},
			},
		},
	}

	newConfig := compat.ConvertPipelineConfig(oldConfig)

	// 验证metadata被保留
	if metadata, ok := newConfig["metadata"].(map[string]interface{}); ok {
		if metadata["custom_field"] != "custom_value" {
			t.Error("metadata custom_field not preserved")
		}
		if metadata["aligned_proxy_mode"] != "transparent" {
			t.Error("metadata aligned_proxy_mode not set correctly")
		}
	} else {
		t.Error("metadata not preserved")
	}
}

func TestIntegration_LegacyConfigWarnings(t *testing.T) {
	compat := NewConfigCompatLayer()

	// 测试旧版配置警告
	tests := []struct {
		name            string
		config          map[string]interface{}
		expectedWarnings int
	}{
		{
			name: "旧版流水线",
			config: map[string]interface{}{
				"id": "transparent-proxy",
			},
			expectedWarnings: 1,
		},
		{
			name: "旧版快捷方式",
			config: map[string]interface{}{
				"id":            "agent-skill-router",
				"shortcut_code": "#skill",
			},
			expectedWarnings: 2,
		},
		{
			name: "新版流水线",
			config: map[string]interface{}{
				"id": "transparent",
			},
			expectedWarnings: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			warnings := compat.ValidateLegacyConfig(tt.config)
			if len(warnings) != tt.expectedWarnings {
				t.Errorf("ValidateLegacyConfig() warnings = %v, want %v", len(warnings), tt.expectedWarnings)
			}
		})
	}
}