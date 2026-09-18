package openai

import "testing"

func TestShouldRotateOpenAIAccount_ModelNotSupported(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		want   bool
	}{
		{"401 modelerror no rotate", 401, `{"error":{"type":"ModelError","message":"Model key-model-A is not supported"}}`, false},
		{"400 model no rotate", 400, `model foo is not supported`, false},
		{"401 plain auth rotates", 401, `{"error":"invalid_api_key"}`, true},
		{"429 rotates", 429, `rate limited`, true},
		{"500 rotates", 500, `internal error`, true},
		{"401 credits rotates", 401, `{"error":{"type":"CreditsError","message":"Insufficient credits"}}`, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldRotateOpenAIAccount(tc.status, tc.body); got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}
