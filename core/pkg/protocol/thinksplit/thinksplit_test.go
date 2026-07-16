package thinksplit

import "testing"

func TestSplit(t *testing.T) {
	tests := []struct {
		name              string
		content           string
		expectedVisible   string
		expectedReasoning string
	}{
		{
			name:              "empty content",
			content:           "",
			expectedVisible:   "",
			expectedReasoning: "",
		},
		{
			name:              "no think tag",
			content:           "Hello, world!",
			expectedVisible:   "Hello, world!",
			expectedReasoning: "",
		},
		{
			name:              "complete think tag",
			content:           "Hello <think>Let me think about this</think> World",
			expectedVisible:   "Hello  World",
			expectedReasoning: "Let me think about this",
		},
		{
			name:              "think tag at start",
			content:           "<think>Let me think</think> Hello",
			expectedVisible:   " Hello",
			expectedReasoning: "Let me think",
		},
		{
			name:              "think tag at end",
			content:           "Hello <think>Let me think</think>",
			expectedVisible:   "Hello ",
			expectedReasoning: "Let me think",
		},
		{
			name:              "only think tag",
			content:           "<think>Let me think</think>",
			expectedVisible:   "",
			expectedReasoning: "Let me think",
		},
		{
			name:              "only open tag - enters reasoning buffer",
			content:           "Hello <think>Let me think",
			expectedVisible:   "Hello ",
			expectedReasoning: "Let me think",
		},
		{
			name:              "multiple think tags - only first processed",
			content:           "<think>First thought</think> Text <think>Second thought</think>",
			expectedVisible:   " Text <think>Second thought</think>",
			expectedReasoning: "First thought",
		},
		{
			name:              "nested think tags - only outer processed",
			content:           "<think>Outer <think>Inner</think> Outer</think>",
			expectedVisible:   "",
			expectedReasoning: "Outer <think>Inner</think> Outer",
		},
		{
			name:              "empty think tag - treat as open tag",
			content:           "Hello <think> World",
			expectedVisible:   "Hello ",
			expectedReasoning: " World",
		},
		{
			name:              "think tag with newlines",
			content:           "Hello <think>Line 1\nLine 2\nLine 3</think> World",
			expectedVisible:   "Hello  World",
			expectedReasoning: "Line 1\nLine 2\nLine 3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			visible, reasoning := Split(tt.content)
			if visible != tt.expectedVisible {
				t.Errorf("Split() visible = %q, want %q", visible, tt.expectedVisible)
			}
			if reasoning != tt.expectedReasoning {
				t.Errorf("Split() reasoning = %q, want %q", reasoning, tt.expectedReasoning)
			}
		})
	}
}

func TestStreamSplitter(t *testing.T) {
	tests := []struct {
		name              string
		chunks            []string
		expectedVisible   string
		expectedReasoning string
	}{
		{
			name:              "single chunk with complete tag",
			chunks:            []string{"Hello <think>Let me think</think> World"},
			expectedVisible:   "Hello  World",
			expectedReasoning: "Let me think",
		},
		{
			name:              "tag split across chunks",
			chunks:            []string{"Hello <think>Let me ", "think about ", "this"},
			expectedVisible:   "Hello ",
			expectedReasoning: "Let me think about this",
		},
		{
			name:              "tag split at boundary",
			chunks:            []string{"Hello <think>Let me think</think>", " World"},
			expectedVisible:   "Hello  World",
			expectedReasoning: "Let me think",
		},
		{
			name:              "multiple chunks without tags",
			chunks:            []string{"Hello ", "World"},
			expectedVisible:   "Hello World",
			expectedReasoning: "",
		},
		{
			name:              "open tag in one chunk, close in another",
			chunks:            []string{"Hello <think>", "Let me think"},
			expectedVisible:   "Hello ",
			expectedReasoning: "Let me think",
		},
		{
			name:              "multiple think tags across chunks - only first processed",
			chunks:            []string{"<think>First", " thought</think> Text <think>Second", " thought</think>"},
			expectedVisible:   " Text <think>Second thought</think>",
			expectedReasoning: "First thought",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			splitter := NewStreamSplitter()
			var visibleAcc, reasoningAcc string

			for _, chunk := range tt.chunks {
				visible, reasoning := splitter.Feed(chunk)
				visibleAcc += visible
				reasoningAcc += reasoning
			}

			// Flush remaining
			visible, reasoning := splitter.Flush()
			visibleAcc += visible
			reasoningAcc += reasoning

			if visibleAcc != tt.expectedVisible {
				t.Errorf("StreamSplitter visible = %q, want %q", visibleAcc, tt.expectedVisible)
			}
			if reasoningAcc != tt.expectedReasoning {
				t.Errorf("StreamSplitter reasoning = %q, want %q", reasoningAcc, tt.expectedReasoning)
			}
		})
	}
}
