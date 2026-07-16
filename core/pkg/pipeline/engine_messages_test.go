package pipeline

import "testing"

func TestPrepareNodeInput_InjectsRequestMessages(t *testing.T) {
	engine := &PipelineEngine{logger: &noopLogger{}}
	execCtx := NewExecutionContext(&AgentPatternPipeline{})
	execCtx.SetVariable("input", "python的优缺点")
	execCtx.SetVariable("messages", []Message{
		{Role: "user", Content: "你好"},
		{Role: "assistant", Content: "你好！"},
		{Role: "user", Content: "python的优缺点"},
	})

	input := engine.prepareNodeInput(PipelineNodeConfig{ID: "generator"}, execCtx)
	if input.Content != "python的优缺点" {
		t.Fatalf("Content = %q, want python的优缺点", input.Content)
	}
	if len(input.Messages) != 3 {
		t.Fatalf("len(Messages) = %d, want 3", len(input.Messages))
	}
	if input.Messages[2].Content != "python的优缺点" {
		t.Fatalf("last message = %q, want python的优缺点", input.Messages[2].Content)
	}
}

type noopLogger struct{}

func (noopLogger) Info(string, ...interface{})  {}
func (noopLogger) Warn(string, ...interface{})  {}
func (noopLogger) Error(string, ...interface{}) {}
func (noopLogger) Debug(string, ...interface{}) {}