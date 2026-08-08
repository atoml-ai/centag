package pipeline

import (
	"testing"
)

func TestTransparentOutputIsEmpty(t *testing.T) {
	cases := []struct {
		name string
		out  *NodeOutput
		want bool
	}{
		{"nil", nil, true},
		{"empty", &NodeOutput{}, true},
		{"content", &NodeOutput{Content: "hi"}, false},
		{"reasoning only", &NodeOutput{ReasoningContent: "think"}, false},
		{"tool calls", &NodeOutput{ToolCalls: []ToolCall{{ID: "t1"}}}, false},
		{"whitespace content", &NodeOutput{Content: "  \n "}, true},
	}
	for _, tc := range cases {
		if got := transparentOutputIsEmpty(tc.out); got != tc.want {
			t.Fatalf("%s: transparentOutputIsEmpty() = %v, want %v", tc.name, got, tc.want)
		}
	}
}
