package pipeline

import (
	"context"
	"testing"
)

func TestRouterNode_KeywordThenIntent_KeywordFirst(t *testing.T) {
	node, err := NewRouterNode(NodeConfig{
		CustomConfig: map[string]interface{}{
			"routing_strategy": "keyword_then_intent",
			"default_route":    "gen_default",
			"routes": map[string]interface{}{
				"code": "gen_code",
				"chat": "gen_chat",
			},
			"intent": map[string]interface{}{
				"enable_fast_matcher":   true,
				"enable_llm_classifier": false,
				"confidence_threshold":  0.55,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	out, err := node.Execute(context.Background(), &NodeInput{Content: "please help me with code review"})
	if err != nil {
		t.Fatal(err)
	}
	// keyword "code" in routes is compiled as contains rule → should hit gen_code
	if out.Metadata["selected_route"] != "gen_code" {
		t.Fatalf("selected_route=%v matched=%v", out.Metadata["selected_route"], out.Metadata["matched"])
	}
}

func TestRouterNode_KeywordThenIntent_IntentFallback(t *testing.T) {
	SetIntentResolver(CategoryKeywordResolver{})
	node, err := NewRouterNode(NodeConfig{
		CustomConfig: map[string]interface{}{
			"routing_strategy": "keyword_then_intent",
			"default_route":    "gen_default",
			"routes": map[string]interface{}{
				"translate": "gen_translate",
				"analysis":  "gen_analysis",
			},
			"intent": map[string]interface{}{
				"enable_fast_matcher":   true,
				"enable_llm_classifier": false,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	// Avoid matching route keys as raw contains in first pass by using longer unique phrasing
	// that still contains category key for intent resolver — rules are built from same keys,
	// so first pass will also match. Use a stub resolver that only returns on exact phrase.
	SetIntentResolver(stubIntentResolver{cat: "analysis", conf: 0.9})
	defer SetIntentResolver(CategoryKeywordResolver{})

	// Content without substring "translate"/"analysis" so rules miss; resolver returns analysis
	out, err := node.Execute(context.Background(), &NodeInput{Content: "please examine this dataset carefully"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Metadata["selected_route"] != "gen_analysis" {
		t.Fatalf("want gen_analysis via intent, got selected=%v matched=%v", out.Metadata["selected_route"], out.Metadata["matched"])
	}
}

func TestRouterNode_KeywordThenIntent_DefaultAndNoLLM(t *testing.T) {
	calls := 0
	SetIntentResolver(stubIntentResolver{cat: "", conf: 0, onCall: func() { calls++ }})
	defer SetIntentResolver(CategoryKeywordResolver{})

	node, err := NewRouterNode(NodeConfig{
		CustomConfig: map[string]interface{}{
			"routing_strategy": "keyword_then_intent",
			"default_route":    "gen_default",
			"routes": map[string]interface{}{
				"code": "gen_code",
			},
			"intent": map[string]interface{}{
				"enable_fast_matcher":   true,
				"enable_llm_classifier": false,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	out, err := node.Execute(context.Background(), &NodeInput{Content: "hello there"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Metadata["selected_route"] != "gen_default" {
		t.Fatalf("want default, got %v", out.Metadata["selected_route"])
	}
	if calls != 1 {
		t.Fatalf("intent resolver should be called once, got %d", calls)
	}
}

type stubIntentResolver struct {
	cat    string
	conf   float64
	onCall func()
}

func (s stubIntentResolver) ResolveCategory(ctx context.Context, content string, categories []string) (string, float64, error) {
	if s.onCall != nil {
		s.onCall()
	}
	return s.cat, s.conf, nil
}
