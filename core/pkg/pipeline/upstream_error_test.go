package pipeline

import (
	"testing"
)

func newGeneratorForTest(backend string) *GeneratorNode {
	node, err := NewGeneratorNode(NodeConfig{Backend: backend, Model: "m"})
	if err != nil {
		panic(err)
	}
	return node.(*GeneratorNode)
}

func TestDetectUpstreamErrorPayload(t *testing.T) {
	cases := []struct {
		name    string
		ct      string
		body    string
		wantHit bool
		wantSt  int
	}{
		{"typed invalid_request", "application/json", `{"error":{"message":"Input must have at least 1 token.","type":"invalid_request_error","code":"invalid_prompt"}}`, true, 400},
		{"auth error", "application/json", `{"error":{"type":"authentication_error","code":"invalid_api_key"}}`, true, 401},
		{"permission", "application/json", `{"error":{"type":"permission_error"}}`, true, 403},
		{"not found", "application/json", `{"error":{"type":"not_found_error"}}`, true, 404},
		{"rate limit", "application/json", `{"error":{"type":"rate_limit_error"}}`, true, 429},
		{"string error", "application/json", `{"error":"not found"}`, true, 502},
		{"unknown type", "application/json", `{"error":{"message":"boom","type":"weird"}}`, true, 502},
		{"normal chat completion", "application/json", `{"id":"x","choices":[{"message":{"content":"hi"}}],"error":null}`, false, 0},
		{"non-json ct", "text/plain", `{"error":"x"}`, false, 0},
		{"non-json body", "application/json", `hello`, false, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			st, hit := DetectUpstreamErrorPayload(tc.ct, []byte(tc.body))
			if hit != tc.wantHit || (hit && st != tc.wantSt) {
				t.Fatalf("got (hit=%v st=%d), want (hit=%v st=%d)", hit, st, tc.wantHit, tc.wantSt)
			}
		})
	}
}

// TestGeneratorMetadataFiltersUnrenderedTemplate 是 P1-T4 的回归：
// backend_id 为未渲染模板串时不得进入节点元数据（防调度内部细节泄漏）。
func TestGeneratorMetadataFiltersUnrenderedTemplate(t *testing.T) {
	n := newGeneratorForTest("{{system.fallback_backend}}")
	out := n.buildGeneratorOutput(&NodeInput{Content: "q"}, &LLMResponse{Content: "ok", Model: "m"}, nil)
	if bid, _ := out.Metadata["backend_id"].(string); bid != "" {
		t.Fatalf("unrendered template leaked into metadata backend_id=%q", bid)
	}
}

func TestGeneratorMetadataKeepsPlainBackend(t *testing.T) {
	n := newGeneratorForTest("real-backend")
	out := n.buildGeneratorOutput(&NodeInput{Content: "q"}, &LLMResponse{Content: "ok", Model: "m"}, nil)
	if bid, _ := out.Metadata["backend_id"].(string); bid != "real-backend" {
		t.Fatalf("plain backend_id lost: %q", bid)
	}
}
