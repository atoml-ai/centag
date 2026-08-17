package pipeline

import (
	"testing"
)

func TestConfigCompatLayer_ConvertPipelineConfig(t *testing.T) {
	compat := NewConfigCompatLayer()

	tests := []struct {
		name           string
		oldConfig      map[string]interface{}
		expectedID     string
		expectedName   string
	}{
		{
			name: "transparent-proxy转换",
			oldConfig: map[string]interface{}{
				"id":            "transparent-proxy",
				"name":          "Transparent Mode",
				"shortcut_code": "#t",
				"nodes": []interface{}{
					map[string]interface{}{
						"id":   "forward",
						"type": "transparent_forward",
						"config": map[string]interface{}{
							"custom_config": map[string]interface{}{
								"route_policy":         "match_model",
								"inject_system_prompt": false,
							},
						},
					},
				},
			},
			expectedID:   "transparent",
			expectedName: "透明模式",
		},
		{
			name: "direct-backend转换",
			oldConfig: map[string]interface{}{
				"id":            "direct-backend",
				"name":          "Direct Backend",
				"shortcut_code": "#d",
				"nodes": []interface{}{
					map[string]interface{}{
						"id":   "forward",
						"type": "transparent_forward",
						"config": map[string]interface{}{
							"custom_config": map[string]interface{}{
								"route_policy":         "fixed",
								"inject_system_prompt": true,
							},
						},
					},
				},
			},
			expectedID:   "transparent",
			expectedName: "透明模式",
		},
		{
			name: "fixed-egress转换",
			oldConfig: map[string]interface{}{
				"id":            "fixed-egress",
				"name":          "Fixed Egress",
				"shortcut_code": "#j",
				"nodes": []interface{}{
					map[string]interface{}{
						"id":   "forward",
						"type": "transparent_forward",
						"config": map[string]interface{}{
							"custom_config": map[string]interface{}{
								"route_policy":         "fixed",
								"inject_system_prompt": false,
							},
						},
					},
				},
			},
			expectedID:   "transparent",
			expectedName: "透明模式",
		},
		{
			name: "router-mode转换",
			oldConfig: map[string]interface{}{
				"id":            "router-mode",
				"name":          "Router Mode",
				"shortcut_code": "#r",
				"nodes": []interface{}{
					map[string]interface{}{
						"id":   "classifier",
						"type": "router",
						"config": map[string]interface{}{
							"custom_config": map[string]interface{}{
								"routing_strategy": "keyword_contains",
							},
						},
					},
				},
			},
			expectedID:   "router-pipeline",
			expectedName: "Router Pipeline",
		},
		{
			name: "agent-skill-router转换",
			oldConfig: map[string]interface{}{
				"id":            "agent-skill-router",
				"name":          "Agent Skill Router",
				"shortcut_code": "#skill",
				"metadata": map[string]interface{}{
					"agent_skill_pipeline": true,
				},
			},
			expectedID:   "centag-ops-router",
			expectedName: "Centag Ops Router",
		},
		{
			name: "cache-hit转换",
			oldConfig: map[string]interface{}{
				"id":            "cache-hit",
				"name":          "Cache Hit",
				"shortcut_code": "#cache",
			},
			expectedID:   "cache-pipeline",
			expectedName: "Cache Pipeline",
		},
		{
			name: "18-rag-mode转换",
			oldConfig: map[string]interface{}{
				"id":            "18-rag-mode",
				"name":          "RAG Mode",
				"shortcut_code": "#rag",
			},
			expectedID:   "cache-pipeline",
			expectedName: "Cache Pipeline",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := compat.ConvertPipelineConfig(tt.oldConfig)

			if result["id"] != tt.expectedID {
				t.Errorf("ID = %v, want %v", result["id"], tt.expectedID)
			}
			if result["name"] != tt.expectedName {
				t.Errorf("Name = %v, want %v", result["name"], tt.expectedName)
			}

			// 验证metadata中保存了原始快捷方式
			if metadata, ok := result["metadata"].(map[string]interface{}); ok {
				originalID, _ := tt.oldConfig["id"].(string)
				expectedShortcut, _ := tt.oldConfig["shortcut_code"].(string)
				if metadata[originalID+"_shortcut"] != expectedShortcut {
					t.Errorf("Metadata shortcut = %v, want %v", metadata[originalID+"_shortcut"], expectedShortcut)
				}
			}
		})
	}
}

func TestConfigCompatLayer_IsLegacyPipeline(t *testing.T) {
	compat := NewConfigCompatLayer()

	tests := []struct {
		name     string
		config   map[string]interface{}
		expected bool
	}{
		{
			name: "旧版流水线",
			config: map[string]interface{}{
				"id": "transparent-proxy",
			},
			expected: true,
		},
		{
			name: "新版流水线",
			config: map[string]interface{}{
				"id": "transparent",
			},
			expected: false,
		},
		{
			name: "transparent-passthrough旧版",
			config: map[string]interface{}{
				"id": "transparent-passthrough",
			},
			expected: true,
		},
		{
			name: "agent-skill-router",
			config: map[string]interface{}{
				"id": "agent-skill-router",
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := compat.IsLegacyPipeline(tt.config)
			if result != tt.expected {
				t.Errorf("IsLegacyPipeline() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestConfigCompatLayer_GetActualPipelineID(t *testing.T) {
	compat := NewConfigCompatLayer()

	tests := []struct {
		name       string
		requestedID string
		expectedID string
	}{
		{
			name:        "transparent-proxy别名",
			requestedID: "transparent-proxy",
			expectedID:  "transparent",
		},
		{
			name:        "direct-backend别名",
			requestedID: "direct-backend",
			expectedID:  "transparent",
		},
		{
			name:        "fixed-egress别名",
			requestedID: "fixed-egress",
			expectedID:  "transparent",
		},
		{
			name:        "router-mode别名",
			requestedID: "router-mode",
			expectedID:  "router-pipeline",
		},
		{
			name:        "agent-skill-router别名",
			requestedID: "agent-skill-router",
			expectedID:  "centag-ops-router",
		},
		{
			name:        "cache-hit别名",
			requestedID: "cache-hit",
			expectedID:  "cache-pipeline",
		},
		{
			name:        "新版ID不变",
			requestedID: "transparent",
			expectedID:  "transparent",
		},
		{
			name:        "transparent-passthrough旧ID映射",
			requestedID: "transparent-passthrough",
			expectedID:  "transparent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := compat.GetActualPipelineID(tt.requestedID)
			if result != tt.expectedID {
				t.Errorf("GetActualPipelineID() = %v, want %v", result, tt.expectedID)
			}
		})
	}
}

func TestConfigCompatLayer_GetActualShortcutCode(t *testing.T) {
	compat := NewConfigCompatLayer()

	tests := []struct {
		name           string
		requestedCode  string
		expectedCode   string
	}{
		{
			name:          "#skill别名",
			requestedCode: "#skill",
			expectedCode:  "#ops",
		},
		{
			name:          "#t不变",
			requestedCode: "#t",
			expectedCode:  "#t",
		},
		{
			name:          "#ops不变",
			requestedCode: "#ops",
			expectedCode:  "#ops",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := compat.GetActualShortcutCode(tt.requestedCode)
			if result != tt.expectedCode {
				t.Errorf("GetActualShortcutCode() = %v, want %v", result, tt.expectedCode)
			}
		})
	}
}

func TestConfigCompatLayer_ValidateLegacyConfig(t *testing.T) {
	compat := NewConfigCompatLayer()

	tests := []struct {
		name            string
		config          map[string]interface{}
		expectedWarnings int
	}{
		{
			name: "旧版流水线警告",
			config: map[string]interface{}{
				"id": "transparent-proxy",
			},
			expectedWarnings: 1,
		},
		{
			name: "旧版快捷方式警告",
			config: map[string]interface{}{
				"id":            "agent-skill-router",
				"shortcut_code": "#skill",
			},
			expectedWarnings: 2,
		},
		{
			name: "新版流水线无警告",
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

func TestConfigCompatLayer_GetPipelineType(t *testing.T) {
	compat := NewConfigCompatLayer()

	tests := []struct {
		name           string
		config         map[string]interface{}
		expectedType   string
	}{
		{
			name: "透传类流水线",
			config: map[string]interface{}{
				"id": "transparent-proxy",
			},
			expectedType: "transparent",
		},
		{
			name: "路由类流水线",
			config: map[string]interface{}{
				"id": "router-mode",
			},
			expectedType: "router",
		},
		{
			name: "缓存类流水线",
			config: map[string]interface{}{
				"id": "cache-hit",
			},
			expectedType: "cache",
		},
		{
			name: "未知类型",
			config: map[string]interface{}{
				"id": "unknown-pipeline",
			},
			expectedType: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := compat.GetPipelineType(tt.config)
			if result != tt.expectedType {
				t.Errorf("GetPipelineType() = %v, want %v", result, tt.expectedType)
			}
		})
	}
}