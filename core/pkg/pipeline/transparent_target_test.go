package pipeline

import "testing"

func TestResolveTransparentTargetURLExplicit(t *testing.T) {
	got, err := ResolveTransparentTargetURL(map[string]interface{}{
		"target_url": "https://api.example.com",
	}, "", "/v1/chat/completions", "https")
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://api.example.com/v1/chat/completions" {
		t.Fatalf("url = %q", got)
	}
}

func TestResolveTransparentTargetURLFromOriginalHost(t *testing.T) {
	got, err := ResolveTransparentTargetURL(map[string]interface{}{
		"original_host": "api.openai.com",
		"original_path": "/v1/chat/completions",
	}, "", "", "https")
	if err != nil {
		t.Fatal(err)
	}
	want := "https://api.openai.com/v1/chat/completions"
	if got != want {
		t.Fatalf("url = %q, want %q", got, want)
	}
}

func TestResolveTransparentTargetURLFromBackend(t *testing.T) {
	ResolveBackendEndpoint = func(backendID string) (*BackendEndpoint, error) {
		if backendID != "openai" {
			t.Fatalf("backendID = %q", backendID)
		}
		return &BackendEndpoint{BaseURL: "https://api.openai.com/v1"}, nil
	}
	defer func() { ResolveBackendEndpoint = nil }()

	got, err := ResolveTransparentTargetURL(map[string]interface{}{
		"backend_id": "openai",
	}, "", "/chat/completions", "https")
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://api.openai.com/v1/chat/completions" {
		t.Fatalf("url = %q", got)
	}
}