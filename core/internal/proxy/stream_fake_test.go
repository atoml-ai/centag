package proxy

import (
	"testing"

	"centag/core/pkg/plugin"
)

func TestStreamFakeAggregator_Feed(t *testing.T) {
	tests := []struct {
		name            string
		chunks          []plugin.StreamChunk
		maxBytes        int64
		wantContent     string
		wantReasoning   string
		wantFinish      string
		wantErr         bool
	}{
		{
			name:     "single chunk",
			chunks:   []plugin.StreamChunk{{Content: "Hello", FinishReason: "stop"}},
			maxBytes: 1024,
			wantContent: "Hello",
			wantFinish:  "stop",
		},
		{
			name: "multiple chunks",
			chunks: []plugin.StreamChunk{
				{Content: "Hello "},
				{Content: "World"},
				{FinishReason: "stop"},
			},
			maxBytes:    1024,
			wantContent: "Hello World",
			wantFinish:  "stop",
		},
		{
			name: "with reasoning",
			chunks: []plugin.StreamChunk{
				{ReasoningContent: "Thinking..."},
				{Content: "Answer"},
			},
			maxBytes:      1024,
			wantContent:   "Answer",
			wantReasoning: "Thinking...",
		},
		{
			name: "exceeds max bytes",
			chunks: []plugin.StreamChunk{
				{Content: "Very long content"},
			},
			maxBytes: 5,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			aggregator := NewStreamFakeAggregator(tt.maxBytes)
			var err error
			for _, chunk := range tt.chunks {
				err = aggregator.Feed(chunk)
				if err != nil {
					break
				}
			}

			if (err != nil) != tt.wantErr {
				t.Errorf("Feed() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				result := aggregator.Result()
				if result.Content != tt.wantContent {
					t.Errorf("Content = %q, want %q", result.Content, tt.wantContent)
				}
				if result.ReasoningContent != tt.wantReasoning {
					t.Errorf("ReasoningContent = %q, want %q", result.ReasoningContent, tt.wantReasoning)
				}
				if result.FinishReason != tt.wantFinish {
					t.Errorf("FinishReason = %q, want %q", result.FinishReason, tt.wantFinish)
				}
			}
		})
	}
}

func TestStreamFakeHandler_IsEnabled(t *testing.T) {
	tests := []struct {
		name     string
		config   StreamFakeConfig
		wantBool bool
	}{
		{
			name:     "enabled by default",
			config:   DefaultStreamFakeConfig(),
			wantBool: true,
		},
		{
			name:     "disabled",
			config:   StreamFakeConfig{Enabled: false},
			wantBool: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewStreamFakeHandler(tt.config)
			if got := handler.IsEnabled(); got != tt.wantBool {
				t.Errorf("IsEnabled() = %v, want %v", got, tt.wantBool)
			}
		})
	}
}
