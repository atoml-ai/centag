package proxy

import "testing"

func TestExtractMessageText(t *testing.T) {
	tests := []struct {
		name string
		in   interface{}
		want string
		ok   bool
	}{
		{"string", "hello #ch", "hello #ch", true},
		{"text array", []interface{}{
			map[string]interface{}{"type": "text", "text": "<memory_context>\n</memory_context>\n\n#ch hi"},
		}, "<memory_context>\n</memory_context>\n\n#ch hi", true},
		{"text object", map[string]interface{}{"type": "text", "text": "#d hi"}, "#d hi", true},
		{"empty array", []interface{}{}, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ExtractMessageText(tt.in)
			if ok != tt.ok || got != tt.want {
				t.Fatalf("ExtractMessageText() = (%q, %v), want (%q, %v)", got, ok, tt.want, tt.ok)
			}
		})
	}
}