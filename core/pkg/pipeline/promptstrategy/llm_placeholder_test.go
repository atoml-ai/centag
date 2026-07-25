package promptstrategy

import (
	"context"
	"testing"
)

func TestNullPromptLLMProcessor_Passthrough(t *testing.T) {
	p := NewNullPromptLLMProcessor()
	if p.Name() != "null" {
		t.Fatalf("Name=%q", p.Name())
	}
	resp, err := p.Process(context.Background(), PromptLLMRequest{
		Stage: "user_check",
		Text:  "hello secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Text != "hello secret" {
		t.Fatalf("Text=%q", resp.Text)
	}
	if resp.Metadata["processor"] != "null" {
		t.Fatalf("Metadata=%v", resp.Metadata)
	}
}
