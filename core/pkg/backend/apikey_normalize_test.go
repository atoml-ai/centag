package backend

import "testing"

func TestNormalizeOpenAICompatibleAPIKey(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"  sk-abc  ", "sk-abc"},
		{"Bearer sk-xyz", "sk-xyz"},
		{"bearer sk-low", "sk-low"},
		{"Bearer  sk-space", "sk-space"},
	}
	for _, tc := range cases {
		if got := NormalizeOpenAICompatibleAPIKey(tc.in); got != tc.want {
			t.Errorf("NormalizeOpenAICompatibleAPIKey(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
