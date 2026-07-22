package anthropic

import (
	"testing"

	"centag/core/pkg/plugin"
	"centag/plugins/protocol/shared"
)

// TestAnthropic_P2_Fields P2 字段测试套件
func TestAnthropic_P2_Fields(t *testing.T) {
	protocol := &Protocol{}
	runner := shared.NewProtocolTestRunner(t, protocol)

	t.Run("top_k", func(t *testing.T) {
		runTopKTests(t, runner)
	})

	t.Run("citations", func(t *testing.T) {
		runCitationsTests(t, runner)
	})

	t.Run("container", func(t *testing.T) {
		runContainerTests(t, runner)
	})

	t.Run("output_config", func(t *testing.T) {
		runOutputConfigTests(t, runner)
	})
}

// runTopKTests top_k 字段测试
func runTopKTests(t *testing.T, runner *shared.ProtocolTestRunner) {
	runner.RunRequestResponseTest(shared.TestCase{
		Name: "top_k解析",
		RequestJSON: `{
			"model": "claude-3-opus-20240229",
			"max_tokens": 1024,
			"messages": [{"role": "user", "content": [{"type": "text", "text": "Hello"}]}],
			"top_k": 40
		}`,
		ValidateReq: func(t *testing.T, req *plugin.ProxyRequest) {
			if req.TopK != 40 {
				t.Errorf("expected top_k=40, got %d", req.TopK)
			}
		},
		MockResponse: &plugin.ProxyResponse{
			Content:    "Hello!",
			Model:      "claude-3-opus-20240229",
			TokensUsed: 10,
			Metadata: map[string]interface{}{
				"prompt_tokens": 5,
			},
		},
	})

	runner.RunRequestResponseTest(shared.TestCase{
		Name: "top_k默认值",
		RequestJSON: `{
			"model": "claude-3-opus-20240229",
			"max_tokens": 1024,
			"messages": [{"role": "user", "content": [{"type": "text", "text": "Hello"}]}]
		}`,
		ValidateReq: func(t *testing.T, req *plugin.ProxyRequest) {
			if req.TopK != 0 {
				t.Errorf("expected top_k=0, got %d", req.TopK)
			}
		},
		MockResponse: &plugin.ProxyResponse{
			Content:    "Hello!",
			Model:      "claude-3-opus-20240229",
			TokensUsed: 10,
			Metadata: map[string]interface{}{
				"prompt_tokens": 5,
			},
		},
	})
}

// runCitationsTests citations 字段测试
func runCitationsTests(t *testing.T, runner *shared.ProtocolTestRunner) {
	runner.RunRequestResponseTest(shared.TestCase{
		Name: "响应citations",
		RequestJSON: `{
			"model": "claude-3-opus-20240229",
			"max_tokens": 1024,
			"messages": [{"role": "user", "content": [{"type": "text", "text": "Hello"}]}]
		}`,
		MockResponse: &plugin.ProxyResponse{
			Content:    "According to the study, this is important.",
			Model:      "claude-3-opus-20240229",
			TokensUsed: 20,
			Metadata: map[string]interface{}{
				"prompt_tokens": 10,
				"citations": []Citation{
					{
						Type:          "char_location",
						CitedText:     "according to the study",
						DocumentIndex: 0,
						DocumentTitle: "Research Paper",
						StartCharIndex: 100,
						EndCharIndex:   125,
					},
				},
			},
		},
		ValidateResp: func(t *testing.T, resp map[string]interface{}) {
			content := resp["content"].([]interface{})
			if len(content) != 1 {
				t.Fatalf("expected 1 content block, got %d", len(content))
			}
			block := content[0].(map[string]interface{})
			citations, ok := block["citations"].([]interface{})
			if !ok || len(citations) == 0 {
				t.Fatal("expected citations in content block")
			}
			citation := citations[0].(map[string]interface{})
			if citation["type"] != "char_location" {
				t.Errorf("expected citation type 'char_location', got '%v'", citation["type"])
			}
			if citation["cited_text"] != "according to the study" {
				t.Errorf("expected cited_text 'according to the study', got '%v'", citation["cited_text"])
			}
		},
	})

	runner.RunRequestResponseTest(shared.TestCase{
		Name: "无citations时省略",
		RequestJSON: `{
			"model": "claude-3-opus-20240229",
			"max_tokens": 1024,
			"messages": [{"role": "user", "content": [{"type": "text", "text": "Hello"}]}]
		}`,
		MockResponse: &plugin.ProxyResponse{
			Content:    "Hello!",
			Model:      "claude-3-opus-20240229",
			TokensUsed: 10,
			Metadata: map[string]interface{}{
				"prompt_tokens": 5,
			},
		},
		ValidateResp: func(t *testing.T, resp map[string]interface{}) {
			content := resp["content"].([]interface{})
			block := content[0].(map[string]interface{})
			if _, ok := block["citations"]; ok {
				t.Error("expected no citations in content block when not provided")
			}
		},
	})
}

// runContainerTests container 字段测试
func runContainerTests(t *testing.T, runner *shared.ProtocolTestRunner) {
	runner.RunRequestResponseTest(shared.TestCase{
		Name: "container解析",
		RequestJSON: `{
			"model": "claude-3-opus-20240229",
			"max_tokens": 1024,
			"messages": [{"role": "user", "content": [{"type": "text", "text": "Hello"}]}],
			"container": {"id": "container-123"}
		}`,
		ValidateReq: func(t *testing.T, req *plugin.ProxyRequest) {
			container, ok := req.Metadata["container"].(*ContainerConfig)
			if !ok {
				t.Fatalf("expected container in metadata, got %T", req.Metadata["container"])
			}
			if container.ID != "container-123" {
				t.Errorf("expected container ID 'container-123', got '%s'", container.ID)
			}
		},
		MockResponse: &plugin.ProxyResponse{
			Content:    "Hello!",
			Model:      "claude-3-opus-20240229",
			TokensUsed: 10,
			Metadata: map[string]interface{}{
				"prompt_tokens": 5,
			},
		},
	})

	runner.RunRequestResponseTest(shared.TestCase{
		Name: "响应container",
		RequestJSON: `{
			"model": "claude-3-opus-20240229",
			"max_tokens": 1024,
			"messages": [{"role": "user", "content": [{"type": "text", "text": "Hello"}]}]
		}`,
		MockResponse: &plugin.ProxyResponse{
			Content:    "Hello!",
			Model:      "claude-3-opus-20240229",
			TokensUsed: 10,
			Metadata: map[string]interface{}{
				"prompt_tokens": 5,
				"container": &ContainerInfo{
					ID:        "container-456",
					ExpiresAt: "2024-12-31T23:59:59Z",
				},
			},
		},
		ValidateResp: func(t *testing.T, resp map[string]interface{}) {
			container, ok := resp["container"].(map[string]interface{})
			if !ok {
				t.Fatal("expected container in response")
			}
			if container["id"] != "container-456" {
				t.Errorf("expected container ID 'container-456', got '%v'", container["id"])
			}
			if container["expires_at"] != "2024-12-31T23:59:59Z" {
				t.Errorf("expected expires_at '2024-12-31T23:59:59Z', got '%v'", container["expires_at"])
			}
		},
	})

	runner.RunRequestResponseTest(shared.TestCase{
		Name: "无container时省略",
		RequestJSON: `{
			"model": "claude-3-opus-20240229",
			"max_tokens": 1024,
			"messages": [{"role": "user", "content": [{"type": "text", "text": "Hello"}]}]
		}`,
		MockResponse: &plugin.ProxyResponse{
			Content:    "Hello!",
			Model:      "claude-3-opus-20240229",
			TokensUsed: 10,
			Metadata: map[string]interface{}{
				"prompt_tokens": 5,
			},
		},
		ValidateResp: func(t *testing.T, resp map[string]interface{}) {
			if _, ok := resp["container"]; ok {
				t.Error("expected no container in response when not provided")
			}
		},
	})
}

// runOutputConfigTests output_config 字段测试
func runOutputConfigTests(t *testing.T, runner *shared.ProtocolTestRunner) {
	runner.RunRequestResponseTest(shared.TestCase{
		Name: "output_config解析",
		RequestJSON: `{
			"model": "claude-3-opus-20240229",
			"max_tokens": 1024,
			"messages": [{"role": "user", "content": [{"type": "text", "text": "Hello"}]}],
			"output_config": {
				"effort": "high",
				"format": {
					"type": "json_schema",
					"schema": {"type": "object", "properties": {"answer": {"type": "string"}}}
				}
			}
		}`,
		ValidateReq: func(t *testing.T, req *plugin.ProxyRequest) {
			outputConfig, ok := req.Metadata["output_config"].(*OutputConfig)
			if !ok {
				t.Fatalf("expected output_config in metadata, got %T", req.Metadata["output_config"])
			}
			if outputConfig.Effort != "high" {
				t.Errorf("expected effort 'high', got '%s'", outputConfig.Effort)
			}
			if outputConfig.Format == nil {
				t.Fatal("expected format in output_config")
			}
			if outputConfig.Format.Type != "json_schema" {
				t.Errorf("expected format type 'json_schema', got '%s'", outputConfig.Format.Type)
			}
		},
		MockResponse: &plugin.ProxyResponse{
			Content:    `{"answer": "Hello!"}`,
			Model:      "claude-3-opus-20240229",
			TokensUsed: 10,
			Metadata: map[string]interface{}{
				"prompt_tokens": 5,
			},
		},
	})

	runner.RunRequestResponseTest(shared.TestCase{
		Name: "output_config仅effort",
		RequestJSON: `{
			"model": "claude-3-opus-20240229",
			"max_tokens": 1024,
			"messages": [{"role": "user", "content": [{"type": "text", "text": "Hello"}]}],
			"output_config": {
				"effort": "medium"
			}
		}`,
		ValidateReq: func(t *testing.T, req *plugin.ProxyRequest) {
			outputConfig, ok := req.Metadata["output_config"].(*OutputConfig)
			if !ok {
				t.Fatalf("expected output_config in metadata, got %T", req.Metadata["output_config"])
			}
			if outputConfig.Effort != "medium" {
				t.Errorf("expected effort 'medium', got '%s'", outputConfig.Effort)
			}
			if outputConfig.Format != nil {
				t.Error("expected no format in output_config")
			}
		},
		MockResponse: &plugin.ProxyResponse{
			Content:    "Hello!",
			Model:      "claude-3-opus-20240229",
			TokensUsed: 10,
			Metadata: map[string]interface{}{
				"prompt_tokens": 5,
			},
		},
	})
}
