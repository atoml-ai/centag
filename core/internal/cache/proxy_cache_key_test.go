package cache

import (
	"testing"

	"centag/core/pkg/logger"
)

func init() {
	logger.Init(logger.Config{
		Level:  "debug",
		Format: "console",
		Output: "stdout",
	})
}

func newTestProxyCache() *ProxyCache {
	return NewProxyCache(nil, true)
}

func TestGetRequestKey_NoUserMessage(t *testing.T) {
	pc := newTestProxyCache()
	key, err := pc.GetRequestKey("gpt-4o", []interface{}{
		map[string]interface{}{"role": "system", "content": "instructions"},
	}, 0.0, 0, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "" {
		t.Errorf("expected empty key when no user message, got %q", key)
	}
}

func TestGetRequestKey_StableForSameInput(t *testing.T) {
	pc := newTestProxyCache()
	messages := []interface{}{
		map[string]interface{}{"role": "system", "content": "sys"},
		map[string]interface{}{"role": "user", "content": "hello"},
	}

	key1, _ := pc.GetRequestKey("gpt-4o", messages, 0.7, 1024, nil, nil, nil)
	key2, _ := pc.GetRequestKey("gpt-4o", messages, 0.7, 1024, nil, nil, nil)

	if key1 == "" {
		t.Fatal("expected non-empty key")
	}
	if key1 != key2 {
		t.Errorf("keys differ for identical input: %q vs %q", key1, key2)
	}
}

func TestGetRequestKey_DiffersByResponseFormat(t *testing.T) {
	pc := newTestProxyCache()
	messages := []interface{}{
		map[string]interface{}{"role": "user", "content": "write json"},
	}

	key1, _ := pc.GetRequestKey("gpt-4o", messages, 0.0, 0,
		map[string]interface{}{"type": "json_object"}, nil, nil)
	key2, _ := pc.GetRequestKey("gpt-4o", messages, 0.0, 0,
		map[string]interface{}{"type": "text"}, nil, nil)

	if key1 == "" || key2 == "" {
		t.Fatal("expected non-empty keys")
	}
	if key1 == key2 {
		t.Error("keys should differ for different response_format")
	}
}

func TestGetRequestKey_DiffersBySeed(t *testing.T) {
	pc := newTestProxyCache()
	messages := []interface{}{
		map[string]interface{}{"role": "user", "content": "random"},
	}

	key1, _ := pc.GetRequestKey("gpt-4o", messages, 0.0, 0, nil, nil, 1)
	key2, _ := pc.GetRequestKey("gpt-4o", messages, 0.0, 0, nil, nil, 2)

	if key1 == "" || key2 == "" {
		t.Fatal("expected non-empty keys")
	}
	if key1 == key2 {
		t.Error("keys should differ for different seed values")
	}
}

func TestGetRequestKey_DiffersByToolChoice(t *testing.T) {
	pc := newTestProxyCache()
	messages := []interface{}{
		map[string]interface{}{"role": "user", "content": "use tools"},
	}

	key1, _ := pc.GetRequestKey("gpt-4o", messages, 0.0, 0, nil, "auto", nil)
	key2, _ := pc.GetRequestKey("gpt-4o", messages, 0.0, 0, nil, "none", nil)

	if key1 == "" || key2 == "" {
		t.Fatal("expected non-empty keys")
	}
	if key1 == key2 {
		t.Error("keys should differ for different tool_choice")
	}
}

func TestGetRequestKey_NilOptionalParamsDontAffectKey(t *testing.T) {
	pc := newTestProxyCache()
	messages := []interface{}{
		map[string]interface{}{"role": "user", "content": "test"},
	}

	key1, _ := pc.GetRequestKey("gpt-4o", messages, 0.0, 0, nil, nil, nil)
	key2, _ := pc.GetRequestKey("gpt-4o", messages, 0.0, 0, nil, nil, nil)

	if key1 != key2 {
		t.Error("keys with all nil optional params should match")
	}
}

func TestGetRequestKey_DiffersByModel(t *testing.T) {
	pc := newTestProxyCache()
	messages := []interface{}{
		map[string]interface{}{"role": "user", "content": "hello"},
	}

	key1, _ := pc.GetRequestKey("gpt-4o", messages, 0.0, 0, nil, nil, nil)
	key2, _ := pc.GetRequestKey("gpt-3.5-turbo", messages, 0.0, 0, nil, nil, nil)

	if key1 == key2 {
		t.Error("keys should differ for different models")
	}
}

func TestGetRequestKey_DiffersByTemperature(t *testing.T) {
	pc := newTestProxyCache()
	messages := []interface{}{
		map[string]interface{}{"role": "user", "content": "hello"},
	}

	key1, _ := pc.GetRequestKey("gpt-4o", messages, 0.1, 0, nil, nil, nil)
	key2, _ := pc.GetRequestKey("gpt-4o", messages, 0.9, 0, nil, nil, nil)

	if key1 == key2 {
		t.Error("keys should differ for different temperature")
	}
}

func TestGetRequestKey_LastUserMessageOnly(t *testing.T) {
	pc := newTestProxyCache()
	messages1 := []interface{}{
		map[string]interface{}{"role": "user", "content": "first"},
		map[string]interface{}{"role": "assistant", "content": "reply"},
		map[string]interface{}{"role": "user", "content": "last"},
	}
	messages2 := []interface{}{
		map[string]interface{}{"role": "user", "content": "different first"},
		map[string]interface{}{"role": "assistant", "content": "reply"},
		map[string]interface{}{"role": "user", "content": "last"},
	}

	key1, _ := pc.GetRequestKey("gpt-4o", messages1, 0.0, 0, nil, nil, nil)
	key2, _ := pc.GetRequestKey("gpt-4o", messages2, 0.0, 0, nil, nil, nil)

	if key1 == "" || key2 == "" {
		t.Fatal("expected non-empty keys")
	}
	if key1 != key2 {
		t.Error("keys should match when only last user messages are identical")
	}
}
