package pipeline

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTransparentForwardNode_RedirectNever(t *testing.T) {
	// 创建一个返回 302 重定向的测试服务器
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "http://example.com/new-location")
		w.WriteHeader(http.StatusFound)
		w.Write([]byte("Redirect"))
	}))
	defer backend.Close()

	config := NodeConfig{
		CustomConfig: map[string]interface{}{
			"redirect_policy": "never",
		},
	}

	node, err := NewTransparentForwardNode(config)
	if err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	if node.(*TransparentForwardNode).RedirectPolicy != "never" {
		t.Errorf("expected redirect_policy 'never', got '%s'", node.(*TransparentForwardNode).RedirectPolicy)
	}
}

func TestTransparentForwardNode_RedirectAlways(t *testing.T) {
	// 创建一个返回 302 重定向的测试服务器
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "http://example.com/new-location")
		w.WriteHeader(http.StatusFound)
		w.Write([]byte("Redirect"))
	}))
	defer backend.Close()

	config := NodeConfig{
		CustomConfig: map[string]interface{}{
			"redirect_policy": "always",
			"max_redirects":   3,
		},
	}

	node, err := NewTransparentForwardNode(config)
	if err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	if node.(*TransparentForwardNode).RedirectPolicy != "always" {
		t.Errorf("expected redirect_policy 'always', got '%s'", node.(*TransparentForwardNode).RedirectPolicy)
	}

	if node.(*TransparentForwardNode).MaxRedirects != 3 {
		t.Errorf("expected max_redirects 3, got %d", node.(*TransparentForwardNode).MaxRedirects)
	}
}

func TestTransparentForwardNode_RedirectSmart(t *testing.T) {
	config := NodeConfig{
		CustomConfig: map[string]interface{}{
			"redirect_policy": "smart",
		},
	}

	node, err := NewTransparentForwardNode(config)
	if err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	if node.(*TransparentForwardNode).RedirectPolicy != "smart" {
		t.Errorf("expected redirect_policy 'smart', got '%s'", node.(*TransparentForwardNode).RedirectPolicy)
	}
}

func TestTransparentForwardNode_DefaultRedirectPolicy(t *testing.T) {
	config := NodeConfig{}

	node, err := NewTransparentForwardNode(config)
	if err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	if node.(*TransparentForwardNode).RedirectPolicy != "never" {
		t.Errorf("expected default redirect_policy 'never', got '%s'", node.(*TransparentForwardNode).RedirectPolicy)
	}

	if node.(*TransparentForwardNode).MaxRedirects != 5 {
		t.Errorf("expected default max_redirects 5, got %d", node.(*TransparentForwardNode).MaxRedirects)
	}
}
