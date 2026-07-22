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
