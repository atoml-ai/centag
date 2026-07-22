package openai

import (
	"testing"

	"centag/core/pkg/plugin"
	"centag/plugins/protocol/shared"
)

// TestOpenAI_P2_Fields P2 字段测试套件
func TestOpenAI_P2_Fields(t *testing.T) {
	protocol := &Protocol{}
	runner := shared.NewProtocolTestRunner(t, protocol)

	t.Run("logprobs", func(t *testing.T) {
		runLogprobsTests(t, runner)
	})

	t.Run("modalities", func(t *testing.T) {
		runModalitiesTests(t, runner)
	})

	t.Run("audio", func(t *testing.T) {
		runAudioTests(t, runner)
	})

	t.Run("store", func(t *testing.T) {
		runStoreTests(t, runner)
	})

	t.Run("metadata", func(t *testing.T) {
		runMetadataTests(t, runner)
	})
}

// runLogprobsTests logprobs 字段测试
func runLogprobsTests(t *testing.T, runner *shared.ProtocolTestRunner) {
	runner.RunRequestResponseTest(shared.TestCase{
		Name: "logprobs解析",
		RequestJSON: `{
			"model": "gpt-4",
			"messages": [{"role": "user", "content": "Hello"}],
			"logprobs": true,
			"top_logprobs": 5
		}`,
		ValidateReq: func(t *testing.T, req *plugin.ProxyRequest) {
			if logprobs, ok := req.Metadata["logprobs"].(bool); !ok || !logprobs {
				t.Errorf("expected logprobs=true in metadata, got %v", req.Metadata["logprobs"])
			}
			if topLogprobs, ok := req.Metadata["top_logprobs"].(int); !ok || topLogprobs != 5 {
				t.Errorf("expected top_logprobs=5 in metadata, got %v", req.Metadata["top_logprobs"])
			}
		},
		MockResponse: &plugin.ProxyResponse{
			Content:    "Hello!",
			Model:      "gpt-4",
			TokensUsed: 10,
			Metadata: map[string]interface{}{
				"prompt_tokens": 5,
			},
		},
	})

	runner.RunRequestResponseTest(shared.TestCase{
		Name: "响应logprobs",
		RequestJSON: `{
			"model": "gpt-4",
			"messages": [{"role": "user", "content": "Hello"}],
			"logprobs": true
		}`,
		MockResponse: &plugin.ProxyResponse{
			Content:    "Hello!",
			Model:      "gpt-4",
			TokensUsed: 10,
			Metadata: map[string]interface{}{
				"prompt_tokens": 5,
				"logprobs": &ChoiceLogprobs{
					Content: []TokenLogprob{
						{
							Token:   "Hello",
							Logprob: -0.5,
							Bytes:   []int{72, 101, 108, 108, 111},
							TopLogprobs: []TopTokenLogprob{
								{Token: "Hello", Logprob: -0.5},
							},
						},
					},
				},
			},
		},
		ValidateResp: func(t *testing.T, resp map[string]interface{}) {
			choices := resp["choices"].([]interface{})
			choice := choices[0].(map[string]interface{})
			logprobs, ok := choice["logprobs"].(map[string]interface{})
			if !ok {
				t.Fatal("expected logprobs in choice")
			}
			content := logprobs["content"].([]interface{})
			if len(content) != 1 {
				t.Fatalf("expected 1 logprob entry, got %d", len(content))
			}
			entry := content[0].(map[string]interface{})
			if entry["token"] != "Hello" {
				t.Errorf("expected token 'Hello', got '%v'", entry["token"])
			}
		},
	})
}

// runModalitiesTests modalities 字段测试
func runModalitiesTests(t *testing.T, runner *shared.ProtocolTestRunner) {
	runner.RunRequestResponseTest(shared.TestCase{
		Name: "modalities解析",
		RequestJSON: `{
			"model": "gpt-4",
			"messages": [{"role": "user", "content": "Hello"}],
			"modalities": ["text", "audio"]
		}`,
		ValidateReq: func(t *testing.T, req *plugin.ProxyRequest) {
			if len(req.Modalities) != 2 {
				t.Fatalf("expected 2 modalities, got %d", len(req.Modalities))
			}
			if req.Modalities[0] != "text" || req.Modalities[1] != "audio" {
				t.Errorf("expected modalities [text, audio], got %v", req.Modalities)
			}
		},
		MockResponse: &plugin.ProxyResponse{
			Content:    "Hello!",
			Model:      "gpt-4",
			TokensUsed: 10,
			Metadata: map[string]interface{}{
				"prompt_tokens": 5,
			},
		},
	})
}

// runAudioTests audio 字段测试
func runAudioTests(t *testing.T, runner *shared.ProtocolTestRunner) {
	runner.RunRequestResponseTest(shared.TestCase{
		Name: "audio解析",
		RequestJSON: `{
			"model": "gpt-4",
			"messages": [{"role": "user", "content": "Hello"}],
			"audio": {"voice": "alloy", "format": "wav"}
		}`,
		ValidateReq: func(t *testing.T, req *plugin.ProxyRequest) {
			audio, ok := req.Metadata["audio"].(map[string]interface{})
			if !ok {
				t.Fatal("expected audio object in metadata")
			}
			if audio["voice"] != "alloy" {
				t.Errorf("expected voice 'alloy', got '%v'", audio["voice"])
			}
			if audio["format"] != "wav" {
				t.Errorf("expected format 'wav', got '%v'", audio["format"])
			}
		},
		MockResponse: &plugin.ProxyResponse{
			Content:    "Hello!",
			Model:      "gpt-4",
			TokensUsed: 10,
			Metadata: map[string]interface{}{
				"prompt_tokens": 5,
			},
		},
	})
}

// runStoreTests store 字段测试
func runStoreTests(t *testing.T, runner *shared.ProtocolTestRunner) {
	runner.RunRequestResponseTest(shared.TestCase{
		Name: "store解析",
		RequestJSON: `{
			"model": "gpt-4",
			"messages": [{"role": "user", "content": "Hello"}],
			"store": true
		}`,
		ValidateReq: func(t *testing.T, req *plugin.ProxyRequest) {
			if store, ok := req.Metadata["store"].(bool); !ok || !store {
				t.Errorf("expected store=true in metadata, got %v", req.Metadata["store"])
			}
		},
		MockResponse: &plugin.ProxyResponse{
			Content:    "Hello!",
			Model:      "gpt-4",
			TokensUsed: 10,
			Metadata: map[string]interface{}{
				"prompt_tokens": 5,
			},
		},
	})
}

// runMetadataTests metadata 字段测试
func runMetadataTests(t *testing.T, runner *shared.ProtocolTestRunner) {
	runner.RunRequestResponseTest(shared.TestCase{
		Name: "metadata解析",
		RequestJSON: `{
			"model": "gpt-4",
			"messages": [{"role": "user", "content": "Hello"}],
			"metadata": {"user_id": "user-123"}
		}`,
		ValidateReq: func(t *testing.T, req *plugin.ProxyRequest) {
			clientMeta, ok := req.Metadata["client_metadata"].(map[string]string)
			if !ok {
				t.Fatalf("expected client_metadata map[string]string in metadata, got %T", req.Metadata["client_metadata"])
			}
			if clientMeta["user_id"] != "user-123" {
				t.Errorf("expected user_id 'user-123', got '%v'", clientMeta["user_id"])
			}
		},
		MockResponse: &plugin.ProxyResponse{
			Content:    "Hello!",
			Model:      "gpt-4",
			TokensUsed: 10,
			Metadata: map[string]interface{}{
				"prompt_tokens": 5,
			},
		},
	})
}
