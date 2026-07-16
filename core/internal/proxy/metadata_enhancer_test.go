package proxy

import (
	"encoding/json"
	"centag/core/pkg/logger"
	"testing"
)

func init() {
	// 初始化日志系统，避免测试时panic
	logger.Init(logger.Config{
		Level:  "debug",
		Format: "console",
		Output: "stdout",
	})
}

func TestMetadataEnhancer_EnhanceResponse(t *testing.T) {
	tests := []struct {
		name     string
		enabled  bool
		response string
		metadata *StrategyMetadata
		wantErr  bool
		check    func(t *testing.T, result []byte)
	}{
		{
			name:    "增强响应 - 添加完整元数据",
			enabled: true,
			response: `{
				"content": "Hello, world!",
				"finish_reason": "stop"
			}`,
			metadata: &StrategyMetadata{
				RequestID:       "test-123",
				CacheStatus:     "hit",
				SelectedBackend: "local-model",
				BackendModel:    "llama3.2:3b",
				SelectionReason: "缓存命中",
				LatencyMs:       50,
				FromCache:       true,
				CacheSimilarity: 0.95,
				TokensUsed:      10,
			},
			wantErr: false,
			check: func(t *testing.T, result []byte) {
				var resp map[string]interface{}
				if err := json.Unmarshal(result, &resp); err != nil {
					t.Fatalf("Failed to unmarshal result: %v", err)
				}

				// 检查元数据字段
				if resp["proxyclaw_request_id"] != "test-123" {
					t.Errorf("proxyclaw_request_id = %v, want test-123", resp["proxyclaw_request_id"])
				}
				if resp["proxyclaw_cache"] != "hit" {
					t.Errorf("proxyclaw_cache = %v, want hit", resp["proxyclaw_cache"])
				}
				if resp["proxyclaw_backend"] != "local-model" {
					t.Errorf("proxyclaw_backend = %v, want local-model", resp["proxyclaw_backend"])
				}
				if resp["proxyclaw_from_cache"] != true {
					t.Errorf("proxyclaw_from_cache = %v, want true", resp["proxyclaw_from_cache"])
				}

				// 检查原始字段仍然存在
				if resp["content"] != "Hello, world!" {
					t.Errorf("content = %v, want Hello, world!", resp["content"])
				}
			},
		},
		{
			name:    "禁用增强 - 返回原始响应",
			enabled: false,
			response: `{
				"content": "Hello, world!",
				"finish_reason": "stop"
			}`,
			metadata: &StrategyMetadata{
				RequestID: "test-123",
			},
			wantErr: false,
			check: func(t *testing.T, result []byte) {
				var resp map[string]interface{}
				if err := json.Unmarshal(result, &resp); err != nil {
					t.Fatalf("Failed to unmarshal result: %v", err)
				}

				// 检查没有元数据字段
				if _, exists := resp["proxyclaw_request_id"]; exists {
					t.Errorf("proxyclaw_request_id should not exist when disabled")
				}

				// 检查原始字段仍然存在
				if resp["content"] != "Hello, world!" {
					t.Errorf("content = %v, want Hello, world!", resp["content"])
				}
			},
		},
		{
			name:    "无效JSON - 返回原始响应",
			enabled: true,
			response: `invalid json`,
			metadata: &StrategyMetadata{
				RequestID: "test-123",
			},
			wantErr: false,
			check: func(t *testing.T, result []byte) {
				// 应该返回原始响应
				if string(result) != `invalid json` {
					t.Errorf("result = %v, want invalid json", string(result))
				}
			},
		},
		{
			name:    "nil元数据 - 添加空元数据",
			enabled: true,
			response: `{"content": "test"}`,
			metadata: nil,
			wantErr:  false,
			check: func(t *testing.T, result []byte) {
				var resp map[string]interface{}
				if err := json.Unmarshal(result, &resp); err != nil {
					t.Fatalf("Failed to unmarshal result: %v", err)
				}

				// 原始字段应该存在
				if resp["content"] != "test" {
					t.Errorf("content = %v, want test", resp["content"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enhancer := NewMetadataEnhancer(tt.enabled)
			result, err := enhancer.EnhanceResponse([]byte(tt.response), tt.metadata)

			if tt.wantErr {
				if err == nil {
					t.Errorf("EnhanceResponse() expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("EnhanceResponse() unexpected error: %v", err)
				}
				tt.check(t, result)
			}
		})
	}
}

func TestMetadataEnhancer_ExtractMetadata(t *testing.T) {
	tests := []struct {
		name    string
		response string
		want    *StrategyMetadata
		wantNil bool
	}{
		{
			name: "提取完整元数据",
			response: `{
				"content": "test",
				"proxyclaw_request_id": "test-123",
				"proxyclaw_cache": "hit",
				"proxyclaw_backend": "local-model",
				"proxyclaw_model": "llama3.2:3b",
				"proxyclaw_reason": "缓存命中",
				"proxyclaw_latency": 50,
				"proxyclaw_from_cache": true,
				"proxyclaw_cache_similarity": 0.95,
				"proxyclaw_tokens": 10
			}`,
			want: &StrategyMetadata{
				RequestID:       "test-123",
				CacheStatus:     "hit",
				SelectedBackend: "local-model",
				BackendModel:    "llama3.2:3b",
				SelectionReason: "缓存命中",
				LatencyMs:       50,
				FromCache:       true,
				CacheSimilarity: 0.95,
				TokensUsed:      10,
			},
			wantNil: false,
		},
		{
			name: "无元数据字段",
			response: `{
				"content": "test",
				"finish_reason": "stop"
			}`,
			wantNil: true,
		},
		{
			name: "部分元数据字段",
			response: `{
				"content": "test",
				"proxyclaw_request_id": "test-123",
				"proxyclaw_cache": "miss"
			}`,
			want: &StrategyMetadata{
				RequestID:   "test-123",
				CacheStatus: "miss",
			},
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enhancer := NewMetadataEnhancer(true)
			metadata, err := enhancer.ExtractMetadata([]byte(tt.response))

			if err != nil {
				t.Errorf("ExtractMetadata() unexpected error: %v", err)
				return
			}

			if tt.wantNil {
				if metadata != nil {
					t.Errorf("ExtractMetadata() = %v, want nil", metadata)
				}
			} else {
				if metadata == nil {
					t.Errorf("ExtractMetadata() = nil, want %v", tt.want)
					return
				}

				if metadata.RequestID != tt.want.RequestID {
					t.Errorf("RequestID = %v, want %v", metadata.RequestID, tt.want.RequestID)
				}
				if metadata.CacheStatus != tt.want.CacheStatus {
					t.Errorf("CacheStatus = %v, want %v", metadata.CacheStatus, tt.want.CacheStatus)
				}
				if metadata.SelectedBackend != tt.want.SelectedBackend {
					t.Errorf("SelectedBackend = %v, want %v", metadata.SelectedBackend, tt.want.SelectedBackend)
				}
				if metadata.BackendModel != tt.want.BackendModel {
					t.Errorf("BackendModel = %v, want %v", metadata.BackendModel, tt.want.BackendModel)
				}
				if metadata.SelectionReason != tt.want.SelectionReason {
					t.Errorf("SelectionReason = %v, want %v", metadata.SelectionReason, tt.want.SelectionReason)
				}
				if metadata.LatencyMs != tt.want.LatencyMs {
					t.Errorf("LatencyMs = %v, want %v", metadata.LatencyMs, tt.want.LatencyMs)
				}
				if metadata.FromCache != tt.want.FromCache {
					t.Errorf("FromCache = %v, want %v", metadata.FromCache, tt.want.FromCache)
				}
				if metadata.CacheSimilarity != tt.want.CacheSimilarity {
					t.Errorf("CacheSimilarity = %v, want %v", metadata.CacheSimilarity, tt.want.CacheSimilarity)
				}
				if metadata.TokensUsed != tt.want.TokensUsed {
					t.Errorf("TokensUsed = %v, want %v", metadata.TokensUsed, tt.want.TokensUsed)
				}
			}
		})
	}
}

func TestMetadataEnhancer_EnhanceStreamChunk(t *testing.T) {
	tests := []struct {
		name        string
		enabled     bool
		isFirstChunk bool
		metadata    *StrategyMetadata
		wantNil     bool
		check       func(t *testing.T, result []byte)
	}{
		{
			name:        "第一个chunk - 添加元数据",
			enabled:     true,
			isFirstChunk: true,
			metadata: &StrategyMetadata{
				RequestID:       "test-123",
				CacheStatus:     "hit",
				SelectedBackend: "local-model",
			},
			wantNil: false,
			check: func(t *testing.T, result []byte) {
				resultStr := string(result)
				if !contains(resultStr, "data: {") {
					t.Errorf("result should contain SSE data prefix")
				}
				if !contains(resultStr, "proxyclaw_request_id") {
					t.Errorf("result should contain proxyclaw_request_id")
				}
				if !contains(resultStr, "test-123") {
					t.Errorf("result should contain request_id")
				}
			},
		},
		{
			name:        "非第一个chunk - 不添加元数据",
			enabled:     true,
			isFirstChunk: false,
			metadata: &StrategyMetadata{
				RequestID: "test-123",
			},
			wantNil: true,
		},
		{
			name:        "禁用增强 - 不添加元数据",
			enabled:     false,
			isFirstChunk: true,
			metadata: &StrategyMetadata{
				RequestID: "test-123",
			},
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enhancer := NewMetadataEnhancer(tt.enabled)
			result, hasData := enhancer.EnhanceStreamChunk(tt.isFirstChunk, tt.metadata)

			if tt.wantNil {
				if hasData || result != nil {
					t.Errorf("EnhanceStreamChunk() should return nil")
				}
			} else {
				if !hasData || result == nil {
					t.Errorf("EnhanceStreamChunk() should return data")
				}
				tt.check(t, result)
			}
		})
	}
}

func TestCreateCacheHitMetadata(t *testing.T) {
	metadata := CreateCacheHitMetadata(
		"test-123",
		"local-model",
		"llama3.2:3b",
		0.95,
		50,
	)

	if metadata == nil {
		t.Fatal("CreateCacheHitMetadata() returned nil")
	}

	if metadata.RequestID != "test-123" {
		t.Errorf("RequestID = %v, want test-123", metadata.RequestID)
	}
	if metadata.CacheStatus != "hit" {
		t.Errorf("CacheStatus = %v, want hit", metadata.CacheStatus)
	}
	if metadata.SelectedBackend != "local-model" {
		t.Errorf("SelectedBackend = %v, want local-model", metadata.SelectedBackend)
	}
	if metadata.BackendModel != "llama3.2:3b" {
		t.Errorf("BackendModel = %v, want llama3.2:3b", metadata.BackendModel)
	}
	if metadata.SelectionReason != "缓存命中" {
		t.Errorf("SelectionReason = %v, want 缓存命中", metadata.SelectionReason)
	}
	if metadata.LatencyMs != 50 {
		t.Errorf("LatencyMs = %v, want 50", metadata.LatencyMs)
	}
	if !metadata.FromCache {
		t.Errorf("FromCache = %v, want true", metadata.FromCache)
	}
	if metadata.CacheSimilarity != 0.95 {
		t.Errorf("CacheSimilarity = %v, want 0.95", metadata.CacheSimilarity)
	}
	if len(metadata.DecisionPath) == 0 {
		t.Errorf("DecisionPath should not be empty")
	}
}

func TestCreateBackendSelectMetadata(t *testing.T) {
	path := []DecisionStep{
		{Step: 1, Action: "cache_check", Description: "检查缓存"},
		{Step: 2, Action: "backend_select", Description: "选择后端"},
	}

	metadata := CreateBackendSelectMetadata(
		"test-123",
		"cloud-model",
		"gpt-3.5-turbo",
		"成本最优",
		200,
		150,
		path,
	)

	if metadata == nil {
		t.Fatal("CreateBackendSelectMetadata() returned nil")
	}

	if metadata.RequestID != "test-123" {
		t.Errorf("RequestID = %v, want test-123", metadata.RequestID)
	}
	if metadata.CacheStatus != "miss" {
		t.Errorf("CacheStatus = %v, want miss", metadata.CacheStatus)
	}
	if metadata.SelectedBackend != "cloud-model" {
		t.Errorf("SelectedBackend = %v, want cloud-model", metadata.SelectedBackend)
	}
	if metadata.SelectionReason != "成本最优" {
		t.Errorf("SelectionReason = %v, want 成本最优", metadata.SelectionReason)
	}
	if metadata.LatencyMs != 200 {
		t.Errorf("LatencyMs = %v, want 200", metadata.LatencyMs)
	}
	if metadata.TokensUsed != 150 {
		t.Errorf("TokensUsed = %v, want 150", metadata.TokensUsed)
	}
	if metadata.FromCache {
		t.Errorf("FromCache = %v, want false", metadata.FromCache)
	}
	if len(metadata.DecisionPath) != len(path) {
		t.Errorf("DecisionPath length = %v, want %v", len(metadata.DecisionPath), len(path))
	}
}

func TestBuildDecisionPath(t *testing.T) {
	steps := []struct {
		Step        int
		Action      string
		Description string
		Data        interface{}
		Duration    int
	}{
		{Step: 1, Action: "cache_check", Description: "检查缓存", Duration: 10},
		{Step: 2, Action: "backend_select", Description: "选择后端", Duration: 20},
	}

	path := BuildDecisionPath(steps)

	if len(path) != 2 {
		t.Errorf("path length = %v, want 2", len(path))
	}

	if path[0].Step != 1 || path[0].Action != "cache_check" {
		t.Errorf("path[0] = %+v, want Step=1, Action=cache_check", path[0])
	}

	if path[1].Step != 2 || path[1].Action != "backend_select" {
		t.Errorf("path[1] = %+v, want Step=2, Action=backend_select", path[1])
	}
}

func TestMetadataEnhancer_MetadataToMap(t *testing.T) {
	enhancer := NewMetadataEnhancer(true)

	tests := []struct {
		name     string
		metadata *StrategyMetadata
		wantKeys []string
	}{
		{
			name: "完整元数据",
			metadata: &StrategyMetadata{
				RequestID:       "test-123",
				CacheStatus:     "hit",
				SelectedBackend: "local-model",
				BackendModel:    "llama3.2:3b",
				SelectionReason: "缓存命中",
				LatencyMs:       50,
				FromCache:       true,
				CacheSimilarity: 0.95,
				TokensUsed:      10,
			},
			wantKeys: []string{
				"proxyclaw_request_id",
				"proxyclaw_cache",
				"proxyclaw_backend",
				"proxyclaw_model",
				"proxyclaw_reason",
				"proxyclaw_latency",
				"proxyclaw_from_cache",
				"proxyclaw_cache_similarity",
				"proxyclaw_tokens",
			},
		},
		{
			name:     "空元数据",
			metadata: &StrategyMetadata{},
			wantKeys: []string{},
		},
		{
			name:     "nil元数据",
			metadata: nil,
			wantKeys: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := enhancer.metadataToMap(tt.metadata)

			if err != nil {
				t.Errorf("metadataToMap() unexpected error: %v", err)
				return
			}

			if len(result) != len(tt.wantKeys) {
				t.Errorf("result length = %v, want %v", len(result), len(tt.wantKeys))
			}

			for _, key := range tt.wantKeys {
				if _, exists := result[key]; !exists {
					t.Errorf("result should contain key: %s", key)
				}
			}
		})
	}
}

// 辅助函数
func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr || len(s) > len(substr) && containsInMiddle(s, substr)
}

func containsInMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
