package proxy

import (
	"testing"

	"centag/core/pkg/plugin"
)

func intPtr(i int) *int       { return &i }
func boolPtr(b bool) *bool     { return &b }

func TestCopyProxyRequestFields_AllP0P1Fields(t *testing.T) {
	src := &plugin.ProxyRequest{
		Model:             "gpt-4o",
		Messages:          []plugin.Message{{Role: "user", Content: "hello"}},
		Temperature:       0.7,
		MaxTokens:         2048,
		Stream:            true,
		RawBody:           map[string]interface{}{"test": true},
		Tools:             []plugin.ToolDefinition{{Type: "function", Function: plugin.FunctionDef{Name: "test_func"}}},
		ToolChoice:        "auto",
		ResponseFormat:    &plugin.ResponseFormatSpec{Type: "json_object"},
		Seed:              intPtr(42),
		N:                 intPtr(1),
		User:              "test-user",
		ParallelToolCalls: boolPtr(true),
		Reasoning:         plugin.ReasoningSpec{Effort: "high"},
	}

	result := copyProxyRequestFields(src)

	if result.Model != src.Model {
		t.Errorf("Model: got %q, want %q", result.Model, src.Model)
	}
	if len(result.Messages) != len(src.Messages) {
		t.Errorf("Messages: got %d, want %d", len(result.Messages), len(src.Messages))
	}
	if result.Temperature != src.Temperature {
		t.Errorf("Temperature: got %f, want %f", result.Temperature, src.Temperature)
	}
	if result.MaxTokens != src.MaxTokens {
		t.Errorf("MaxTokens: got %d, want %d", result.MaxTokens, src.MaxTokens)
	}
	if result.Stream != src.Stream {
		t.Errorf("Stream: got %v, want %v", result.Stream, src.Stream)
	}
	if result.Tools == nil || len(result.Tools) != 1 || result.Tools[0].Function.Name != "test_func" {
		t.Errorf("Tools not copied correctly")
	}
	if result.ToolChoice != "auto" {
		t.Errorf("ToolChoice: got %v, want auto", result.ToolChoice)
	}
	if result.ResponseFormat == nil || result.ResponseFormat.Type != "json_object" {
		t.Errorf("ResponseFormat not copied correctly")
	}
	if result.Seed == nil || *result.Seed != 42 {
		t.Errorf("Seed not copied correctly")
	}
	if result.N == nil || *result.N != 1 {
		t.Errorf("N not copied correctly")
	}
	if result.User != "test-user" {
		t.Errorf("User: got %q, want test-user", result.User)
	}
	if result.ParallelToolCalls == nil || *result.ParallelToolCalls != true {
		t.Errorf("ParallelToolCalls not copied correctly")
	}
	if result.Reasoning.Effort != "high" {
		t.Errorf("Reasoning: got %q, want high", result.Reasoning.Effort)
	}
	if result.RawBody == nil {
		t.Error("RawBody should not be nil when source has one")
	}
}

func TestCopyProxyRequestFields_NilFields(t *testing.T) {
	src := &plugin.ProxyRequest{
		Model:    "gpt-4o",
		Messages: []plugin.Message{{Role: "user", Content: "hi"}},
	}

	result := copyProxyRequestFields(src)

	if result.RawBody != nil {
		t.Error("RawBody should be nil")
	}
	if result.Tools != nil {
		t.Error("Tools should be nil")
	}
	if result.ResponseFormat != nil {
		t.Error("ResponseFormat should be nil")
	}
	if result.Seed != nil {
		t.Error("Seed should be nil")
	}
	if result.N != nil {
		t.Error("N should be nil")
	}
	if result.ParallelToolCalls != nil {
		t.Error("ParallelToolCalls should be nil")
	}
}
