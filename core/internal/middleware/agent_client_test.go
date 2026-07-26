package middleware

import "testing"

func TestDetectAgentType(t *testing.T) {
	tests := []struct {
		name         string
		path         string
		ua           string
		explicit     string
		wantType     string
		wantDetected string
	}{
		{
			name:         "explicit header wins",
			path:         "/v1/chat/completions",
			ua:           "Claude-Code/1.0",
			explicit:     "codex",
			wantType:     "codex",
			wantDetected: detectByExplicit,
		},
		{
			name:         "messages fallback to claude",
			path:         "/v1/messages",
			ua:           "curl/8.0",
			wantType:     "claude-code",
			wantDetected: detectByPathFallback,
		},
		{
			name:         "codex from user agent",
			path:         "/v1/chat/completions",
			ua:           "Codex-CLI/0.9",
			wantType:     "codex",
			wantDetected: detectByUserAgent,
		},
		{
			name:         "gemini from user agent",
			path:         "/v1/chat/completions",
			ua:           "Gemini-CLI/1.2",
			wantType:     "gemini-cli",
			wantDetected: detectByUserAgent,
		},
		{
			name:         "pi from user agent prefix",
			path:         "/v1/chat/completions",
			ua:           "pi/0.82.1",
			wantType:     "pi",
			wantDetected: detectByUserAgent,
		},
		{
			name:         "pi from package user agent",
			path:         "/v1/chat/completions",
			ua:           "pi-coding-agent/0.82.1",
			wantType:     "pi",
			wantDetected: detectByUserAgent,
		},
		{
			name:         "pi from explicit header",
			path:         "/v1/chat/completions",
			ua:           "curl/8.0",
			explicit:     "pi",
			wantType:     "pi",
			wantDetected: detectByExplicit,
		},
		{
			name:     "unknown returns empty",
			path:     "/v1/chat/completions",
			ua:       "my-client/1.0",
			wantType: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotType, gotDetected := DetectAgentType(tc.path, tc.ua, tc.explicit)
			if gotType != tc.wantType {
				t.Fatalf("agent type = %q, want %q", gotType, tc.wantType)
			}
			if gotDetected != tc.wantDetected {
				t.Fatalf("detectedBy = %q, want %q", gotDetected, tc.wantDetected)
			}
		})
	}
}
