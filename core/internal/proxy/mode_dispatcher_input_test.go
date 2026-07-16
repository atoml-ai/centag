package proxy

import (
	"testing"

	"centag/core/pkg/plugin"
)

func TestExtractQuestionFromMessages_MultiTurn(t *testing.T) {
	messages := []plugin.Message{
		{Role: "user", Content: "你好"},
		{Role: "assistant", Content: "你好！"},
		{Role: "user", Content: "python的优缺点"},
	}
	got := extractQuestionFromMessages(messages)
	if got != "python的优缺点" {
		t.Fatalf("extractQuestionFromMessages() = %q, want %q", got, "python的优缺点")
	}
}

func TestExtractQuestionFromMessages_SingleTurn(t *testing.T) {
	messages := []plugin.Message{
		{Role: "user", Content: "#a python的优缺点"},
	}
	got := extractQuestionFromMessages(messages)
	if got != "#a python的优缺点" {
		t.Fatalf("extractQuestionFromMessages() = %q, want %q", got, "#a python的优缺点")
	}
}