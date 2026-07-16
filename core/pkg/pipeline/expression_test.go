package pipeline

import (
	"errors"
	"testing"
)

func TestParseExpression(t *testing.T) {
	tests := []struct {
		name        string
		expr        string
		wantBase    string
		wantPath   string
		wantErr    bool
		wantFilter []ExprFilterType
		wantDefVal interface{}
	}{
		{
			name:      "simple field reference",
			expr:      "{{.content}}",
			wantBase:  ".content",
			wantPath:  "content",
			wantErr:  false,
		},
		{
			name:      "simple field with spaces",
			expr:      "{{ .content }}",
			wantBase:  ".content",
			wantPath:  "content",
			wantErr:  false,
		},
		{
			name:      "default filter with string",
			expr:      "{{.field | default \"value\"}}",
			wantBase:  ".field",
			wantPath: "field",
			wantErr:  false,
			wantFilter: []ExprFilterType{FilterDefault},
			wantDefVal: "value",
		},
		{
			name:      "required filter",
			expr:      "{{.field | required}}",
			wantBase:  ".field",
			wantPath: "field",
			wantErr:  false,
			wantFilter: []ExprFilterType{FilterRequired},
		},
		{
			name:      "strict filter",
			expr:      "{{.field | strict}}",
			wantBase:  ".field",
			wantPath: "field",
			wantErr:  false,
			wantFilter: []ExprFilterType{FilterStrict},
		},
		{
			name:      "nested field",
			expr:      "{{.metadata.key}}",
			wantBase:  ".metadata.key",
			wantPath: "metadata.key",
			wantErr:  false,
		},
		{
			name:      "invalid expression",
			expr:      "{{invalid}",
			wantErr:  true,
		},
		{
			name:      "unknown filter",
			expr:      "{{.field | unknown}}",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := ParseExpression(tt.expr)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseExpression() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if parsed.BasePath != tt.wantBase {
				t.Errorf("BasePath = %v, want %v", parsed.BasePath, tt.wantBase)
			}
			if parsed.FieldPath != tt.wantPath {
				t.Errorf("FieldPath = %v, want %v", parsed.FieldPath, tt.wantPath)
			}
			if len(parsed.Filters) != len(tt.wantFilter) {
				t.Errorf("Filters = %v, want %v", parsed.Filters, tt.wantFilter)
			}
			if tt.wantDefVal != nil && parsed.DefaultVal != tt.wantDefVal {
				t.Errorf("DefaultVal = %v, want %v", parsed.DefaultVal, tt.wantDefVal)
			}
		})
	}
}

func TestExpressionParser_EvaluateExpression(t *testing.T) {
	tests := []struct {
		name    string
		data    interface{}
		expr    string
		want    interface{}
		wantErr bool
		errMsg  string
	}{
		{
			name:    "simple content field",
			data:    map[string]interface{}{"content": "hello"},
			expr:    "{{.content}}",
			want:    "hello",
		},
		{
			name:    "missing field without default",
			data:    map[string]interface{}{"content": "hello"},
			expr:    "{{.missing}}",
			want:    nil,
		},
		{
			name:    "missing field with default",
			data:    map[string]interface{}{"content": "hello"},
			expr:    "{{.missing | default \"fallback\"}}",
			want:    "fallback",
		},
		{
			name:    "missing field with required",
			data:    map[string]interface{}{"content": "hello"},
			expr:    "{{.missing | required}}",
			wantErr: true,
			errMsg:  ErrCodeMissingField,
		},
		{
			name:    "missing field with strict",
			data:    map[string]interface{}{"content": "hello"},
			expr:    "{{.missing | strict}}",
			wantErr: true,
			errMsg:  ErrCodeStrict,
		},
		{
			name:    "existing field with strict",
			data:    map[string]interface{}{"content": "hello"},
			expr:    "{{.content | strict}}",
			want:    "hello",
		},
		{
			name:    "nested field",
			data:    map[string]interface{}{"metadata": map[string]interface{}{"key": "value"}},
			expr:    "{{.metadata.key}}",
			want:    "value",
		},
		{
			name:    "nil value with default",
			data:    map[string]interface{}{"field": nil},
			expr:    "{{.field | default \"fallback\"}}",
			want:    "fallback",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewExpressionParser(tt.data)
			val, err := parser.EvaluateExpression(tt.expr)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("EvaluateExpression() want error containing %s", tt.errMsg)
				}
				if exprErr, ok := err.(*ExpressionError); ok && exprErr.Code != tt.errMsg {
					t.Errorf("Error code = %v, want %v", exprErr.Code, tt.errMsg)
				}
				return
			}
			if err != nil {
				t.Fatalf("EvaluateExpression() unexpected error = %v", err)
			}
			if val != tt.want {
				t.Errorf("got = %v (%T), want %v (%T)", val, val, tt.want, tt.want)
			}
		})
	}
}

func TestExpressionParser_SetStrict(t *testing.T) {
	parser := NewExpressionParser(map[string]interface{}{"content": "hello"})

	parser.SetStrict(false)
	val, err := parser.EvaluateExpression("{{.missing}}")
	if err != nil {
		t.Errorf("strict=false, got error = %v", err)
	}
	if val != nil {
		t.Errorf("strict=false, want nil, got %v", val)
	}

	parser.SetStrict(true)
	_, err = parser.EvaluateExpression("{{.missing}}")
	if err == nil {
		t.Errorf("strict=true, want error, got nil")
	}
	if exprErr, ok := err.(*ExpressionError); !ok || exprErr.Code != ErrCodeStrict {
		t.Errorf("strict=true, want strict error code")
	}
}

func TestIsExpression(t *testing.T) {
	tests := []struct {
		input string
		want bool
	}{
		{"{{.content}}", true},
		{"{{ .field }}", true},
		{"not an expression", false},
		{"{{", false},
		{"}}", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := IsExpression(tt.input); got != tt.want {
				t.Errorf("IsExpression(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestNodeInput_ToMap(t *testing.T) {
	ni := &NodeInput{
		Content:  "test content",
		Messages: []Message{{Role: "user", Content: "hello"}},
		Metadata: map[string]interface{}{"key": "value"},
		Context:  map[string]interface{}{"ctxKey": "ctxVal"},
	}

	m := ni.ToMap()

	if m["content"] != "test content" {
		t.Errorf("content = %v, want test content", m["content"])
	}
	if len(m["messages"].([]Message)) != 1 {
		t.Errorf("messages len = %d, want 1", len(m["messages"].([]Message)))
	}
	if m["key"] != "value" {
		t.Errorf("metadata key = %v, want value", m["key"])
	}
	if m["ctxKey"] != "ctxVal" {
		t.Errorf("context key = %v, want ctxVal", m["ctxKey"])
	}
}

func TestProcessNodeInputs(t *testing.T) {
	execCtx := &ExecutionContext{
		pipeline:  &AgentPatternPipeline{ID: "test-pipeline"},
		variables: make(map[string]interface{}),
		results:   make(map[string]*NodeOutput),
	}

	t.Run("basic inputs", func(t *testing.T) {
		nodeInput := &NodeInput{
			Content:  "test question",
			Metadata: map[string]interface{}{"extra": "data"},
		}

		inputs := map[string]string{
			"question": "{{.content}}",
			"extra":    "{{.extra}}",
		}

		result, err := ProcessNodeInputs(nodeInput, execCtx, inputs, false)
		if err != nil {
			t.Fatalf("ProcessNodeInputs() error = %v", err)
		}
		if result["question"] != "test question" {
			t.Errorf("question = %v, want test question", result["question"])
		}
		if result["extra"] != "data" {
			t.Errorf("extra = %v, want data", result["extra"])
		}
	})

	t.Run("with default value", func(t *testing.T) {
		nodeInput := &NodeInput{
			Content:  "test",
			Metadata: nil,
		}

		inputs := map[string]string{
			"field": "{{.missing | default \"fallback\"}}",
		}

		result, err := ProcessNodeInputs(nodeInput, execCtx, inputs, false)
		if err != nil {
			t.Fatalf("ProcessNodeInputs() error = %v", err)
		}
		if result["field"] != "fallback" {
			t.Errorf("field = %v, want fallback", result["field"])
		}
	})

	t.Run("strict mode rejects missing", func(t *testing.T) {
		nodeInput := &NodeInput{
			Content: "test",
		}

		inputs := map[string]string{
			"field": "{{.missing}}",
		}

		_, err := ProcessNodeInputs(nodeInput, execCtx, inputs, true)
		if err == nil {
			t.Fatalf("ProcessNodeInputs() want error, got nil")
		}
		if exprErr, ok := err.(*ExpressionError); !ok {
			t.Fatalf("want ExpressionError, got %T", err)
		} else if exprErr.Code != ErrCodeStrict {
			t.Errorf("error code = %v, want EXPR_STRICT_FAILED", exprErr.Code)
		}
	})

	t.Run("strict mode skips missing with default", func(t *testing.T) {
		nodeInput := &NodeInput{
			Content: "test",
		}

		inputs := map[string]string{
			"field": "{{.missing | default \"fallback\"}}",
		}

		result, err := ProcessNodeInputs(nodeInput, execCtx, inputs, true)
		if err != nil {
			t.Fatalf("ProcessNodeInputs() error = %v", err)
		}
		if result["field"] != "fallback" {
			t.Errorf("field = %v, want fallback", result["field"])
		}
	})
}

func TestExpressionError(t *testing.T) {
	err := &ExpressionError{
		Code:       ErrCodeMissingField,
		Message:    "required field is missing",
		FieldPath:  "user.id",
		Expression: "{{.user.id | required}}",
	}

	want := "[EXPR_MISSING_FIELD] required field is missing (field: user.id, expression: {{.user.id | required}})"
	if err.Error() != want {
		t.Errorf("Error() = %v, want %v", err.Error(), want)
	}

	if !errors.Is(err, ErrMissingRequiredField) {
		t.Errorf("errors.Is() should return true for ErrMissingRequiredField")
	}
}

func TestContextPath(t *testing.T) {
	execCtx := &ExecutionContext{
		pipeline:  &AgentPatternPipeline{ID: "test-pipeline"},
		variables: map[string]interface{}{
			"user_id":    "user123",
			"session_id": "sess456",
		},
		results: make(map[string]*NodeOutput),
	}

	tests := []struct {
		name  string
		expr  string
		want  interface{}
	}{
		{"pipeline_id", "{{context.pipeline_id}}", "test-pipeline"},
		{"user_id", "{{context.user_id}}", "user123"},
		{"session_id", "{{context.session_id}}", "sess456"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nodeInput := &NodeInput{Content: "test"}
			parser := NewExpressionParserWithContext(nodeInput.ToMap(), execCtx)

			val, err := parser.EvaluateExpression(tt.expr)
			if err != nil {
				t.Fatalf("EvaluateExpression() error = %v", err)
			}
			if val != tt.want {
				t.Errorf("got = %v, want %v", val, tt.want)
			}
		})
	}
}