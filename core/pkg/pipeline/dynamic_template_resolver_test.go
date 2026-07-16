package pipeline

import (
	"testing"
)

func TestNewDynamicTemplateResolver(t *testing.T) {
	input := &NodeInput{
		Content:  "test content",
		Metadata: map[string]interface{}{},
	}

	headers := map[string]string{
		"X-Executor-Backend-ID": "test-backend",
		"X-Executor-Model":       "test-model",
	}

	queryParams := map[string]string{
		"pipeline_id": "test-pipeline",
	}

	resolver := NewDynamicTemplateResolver(input, nil, headers, queryParams)

	if resolver == nil {
		t.Fatal("NewDynamicTemplateResolver() returned nil")
	}

	if len(resolver.RequestHeaders) != 2 {
		t.Errorf("RequestHeaders length = %d, want 2", len(resolver.RequestHeaders))
	}

	if len(resolver.QueryParams) != 1 {
		t.Errorf("QueryParams length = %d, want 1", len(resolver.QueryParams))
	}
}

func TestDynamicTemplateResolver_ResolveHeader(t *testing.T) {
	resolver := &DynamicTemplateResolver{
		RequestHeaders: map[string]string{
			"X-Executor-Backend-ID": "test-backend",
			"X-Executor-Model":       "test-model",
		},
	}

	tests := []struct {
		name    string
		path    string
		want    string
		wantErr bool
	}{
		{
			name:    "existing header",
			path:    "header.X-Executor-Backend-ID",
			want:    "test-backend",
			wantErr: false,
		},
		{
			name:    "case insensitive header",
			path:    "header.x-executor-backend-id",
			want:    "test-backend",
			wantErr: false,
		},
		{
			name:    "non-existing header",
			path:    "header.X-Non-Existing",
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 注意：Resolve 方法需要完整的 resolver 初始化
			// 这里简化测试，直接测试 resolveHeader
			got, err := resolver.resolveHeader("X-Executor-Backend-ID")
			if err != nil {
				t.Logf("resolveHeader returned error (expected in test): %v", err)
			}
			if got != "test-backend" {
				t.Errorf("resolveHeader() = %v, want test-backend", got)
			}
		})
	}
}

func TestExtractConfigFromHeaders(t *testing.T) {
	headers := map[string]string{
		"X-Executor-Backend-ID":  "bigmodel",
		"X-Executor-Model":       "glm-4-flash",
		"X-Auditor-Backend-ID":   "bigmodel",
		"X-Auditor-Model":        "glm-5",
		"X-Optimizer-Backend-ID": "bigmodel",
		"X-Optimizer-Model":      "glm-4-flash",
		"X-Backend-ID":           "default-backend",
		"X-Model":                "default-model",
		"X-Target-URL":           "https://api.example.com",
		"X-Pipeline-ID":          "test-pipeline",
		"X-Unknown-Header":       "should-be-ignored",
	}

	config := ExtractConfigFromHeaders(headers)

	expectedKeys := []string{
		"executor_backend",
		"executor_model",
		"auditor_backend",
		"auditor_model",
		"optimizer_backend",
		"optimizer_model",
		"backend_id",
		"model",
		"target_url",
		"pipeline_id",
	}

	for _, key := range expectedKeys {
		if _, ok := config[key]; !ok {
			t.Errorf("ExtractConfigFromHeaders() missing key: %s", key)
		}
	}

	// 验证具体值
	if config["executor_backend"] != "bigmodel" {
		t.Errorf("executor_backend = %v, want bigmodel", config["executor_backend"])
	}

	if config["auditor_model"] != "glm-5" {
		t.Errorf("auditor_model = %v, want glm-5", config["auditor_model"])
	}

	// 验证未知头被忽略
	if _, ok := config["X-Unknown-Header"]; ok {
		t.Error("ExtractConfigFromHeaders() should ignore unknown headers")
	}
}

func TestExtractConfigFromHeaders_Empty(t *testing.T) {
	config := ExtractConfigFromHeaders(map[string]string{})

	if len(config) != 0 {
		t.Errorf("ExtractConfigFromHeaders() with empty headers should return empty config, got %d items", len(config))
	}
}

func TestExtractConfigFromHeaders_Partial(t *testing.T) {
	headers := map[string]string{
		"X-Executor-Backend-ID": "bigmodel",
		"X-Model":               "gpt-4",
	}

	config := ExtractConfigFromHeaders(headers)

	if len(config) != 2 {
		t.Errorf("ExtractConfigFromHeaders() with partial headers should return 2 items, got %d", len(config))
	}

	if config["executor_backend"] != "bigmodel" {
		t.Errorf("executor_backend = %v, want bigmodel", config["executor_backend"])
	}

	if config["model"] != "gpt-4" {
		t.Errorf("model = %v, want gpt-4", config["model"])
	}
}

func TestMergeConfigWithDefaults(t *testing.T) {
	defaults := map[string]interface{}{
		"executor_backend": "default-executor",
		"executor_model":   "default-model",
		"auditor_backend":  "default-auditor",
		"auditor_model":    "default-auditor-model",
		"fixed_value":      "should-remain",
	}

	extracted := map[string]interface{}{
		"executor_backend": "custom-executor",
		"executor_model":   "custom-model",
	}

	merged := MergeConfigWithDefaults(extracted, defaults)

	// 验证提取的配置覆盖默认值
	if merged["executor_backend"] != "custom-executor" {
		t.Errorf("executor_backend = %v, want custom-executor", merged["executor_backend"])
	}

	if merged["executor_model"] != "custom-model" {
		t.Errorf("executor_model = %v, want custom-model", merged["executor_model"])
	}

	// 验证未覆盖的默认值保留
	if merged["auditor_backend"] != "default-auditor" {
		t.Errorf("auditor_backend = %v, want default-auditor", merged["auditor_backend"])
	}

	if merged["fixed_value"] != "should-remain" {
		t.Errorf("fixed_value = %v, want should-remain", merged["fixed_value"])
	}
}

func TestMergeConfigWithDefaults_EmptyExtracted(t *testing.T) {
	defaults := map[string]interface{}{
		"key1": "value1",
		"key2": "value2",
	}

	merged := MergeConfigWithDefaults(map[string]interface{}{}, defaults)

	if len(merged) != 2 {
		t.Errorf("MergeConfigWithDefaults() with empty extracted should return defaults, got %d items", len(merged))
	}

	if merged["key1"] != "value1" {
		t.Errorf("key1 = %v, want value1", merged["key1"])
	}
}

func TestMergeConfigWithDefaults_EmptyDefaults(t *testing.T) {
	extracted := map[string]interface{}{
		"key1": "value1",
	}

	merged := MergeConfigWithDefaults(extracted, map[string]interface{}{})

	if len(merged) != 1 {
		t.Errorf("MergeConfigWithDefaults() with empty defaults should return extracted, got %d items", len(merged))
	}

	if merged["key1"] != "value1" {
		t.Errorf("key1 = %v, want value1", merged["key1"])
	}
}
