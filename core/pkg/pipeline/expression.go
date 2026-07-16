package pipeline

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

const (
	ErrCodeInvalidExpression = "EXPR_INVALID_EXPRESSION"
	ErrCodeMissingField   = "EXPR_MISSING_FIELD"
	ErrCodeInvalidPath   = "EXPR_INVALID_PATH"
	ErrCodeTypeError   = "EXPR_TYPE_ERROR"
	ErrCodeStrict    = "EXPR_STRICT_FAILED"
)

var (
	ErrMissingRequiredField = errors.New("required field is missing")
	ErrStrictModeFailed = errors.New("strict mode: binding failed, aborting pipeline")
)

type ExpressionError struct {
	Code       string
	Message    string
	FieldPath  string
	Expression string
}

func (e *ExpressionError) Error() string {
	if e.FieldPath != "" {
		return fmt.Sprintf("[%s] %s (field: %s, expression: %s)", e.Code, e.Message, e.FieldPath, e.Expression)
	}
	return fmt.Sprintf("[%s] %s (expression: %s)", e.Code, e.Message, e.Expression)
}

func (e *ExpressionError) Unwrap() error {
	switch e.Code {
	case ErrCodeMissingField:
		return ErrMissingRequiredField
	case ErrCodeStrict:
		return ErrStrictModeFailed
	default:
		return errors.New(e.Message)
	}
}

type ExprFilterType int

const (
	FilterNone ExprFilterType = iota
	FilterDefault
	FilterRequired
	FilterStrict
	FilterDefaultStrict
)

type ParsedExpression struct {
	BasePath     string
	Filters     []ExprFilterType
	DefaultVal  interface{}
	FieldPath   string
}

type ExpressionParser struct {
	data      interface{}
	strict    bool
	execCtx   *ExecutionContext
}

func NewExpressionParser(data interface{}) *ExpressionParser {
	return &ExpressionParser{data: data, strict: false}
}

func NewExpressionParserWithContext(data interface{}, execCtx *ExecutionContext) *ExpressionParser {
	return &ExpressionParser{data: data, execCtx: execCtx}
}

func (p *ExpressionParser) SetStrict(strict bool) {
	p.strict = strict
}

var (
	exprRegex    = regexp.MustCompile(`^\{\{\s*(\S+?)\s*(?:\|(.+?))?\}\}$`)
	filterRegex  = regexp.MustCompile(`(\w+)(?:\s+(.+))?`)
)

func ParseExpression(expr string) (*ParsedExpression, error) {
	expr = strings.TrimSpace(expr)
	matches := exprRegex.FindStringSubmatch(expr)
	if len(matches) == 0 {
		return nil, &ExpressionError{
			Code:       ErrCodeInvalidExpression,
			Message:    "invalid expression format",
			Expression: expr,
		}
	}

	parsed := &ParsedExpression{
		BasePath:   matches[1],
		Filters:   make([]ExprFilterType, 0),
		FieldPath:  matches[1],
	}

	if len(matches) > 2 && matches[2] != "" {
		filters := matches[2]
		filterMatches := filterRegex.FindAllStringSubmatch(filters, -1)
		for _, fm := range filterMatches {
			filterName := strings.TrimSpace(fm[1])
			hasValue := len(fm) > 2 && fm[2] != "" && strings.TrimSpace(fm[2]) != ""
			switch filterName {
			case "default":
				parsed.Filters = append(parsed.Filters, FilterDefault)
				if hasValue {
					parsed.DefaultVal = parseDefaultValue(fm[2])
				}
			case "required":
				parsed.Filters = append(parsed.Filters, FilterRequired)
			case "strict":
				parsed.Filters = append(parsed.Filters, FilterStrict)
			default:
				return nil, &ExpressionError{
					Code:        ErrCodeInvalidExpression,
					Message:     fmt.Sprintf("unknown filter: %s", filterName),
					Expression: expr,
				}
			}
		}
	}

	parsed.FieldPath = normalizeFieldPath(parsed.BasePath)

	return parsed, nil
}

func normalizeFieldPath(path string) string {
	if strings.Contains(path, "[") {
		return path
	}
	parts := strings.Split(path, ".")
	var cleaned []string
	for _, part := range parts {
		if part != "" {
			cleaned = append(cleaned, part)
		}
	}
	return strings.Join(cleaned, ".")
}

func parseFieldPathParts(fieldPath string) []string {
	var parts []string
	var current []rune
	for i, r := range fieldPath {
		if r == '.' {
			if len(current) > 0 {
				parts = append(parts, string(current))
				current = nil
			}
		} else if r == '[' {
			if len(current) > 0 {
				parts = append(parts, string(current))
				current = nil
			}
			end := strings.Index(fieldPath[i:], "]")
			if end > 0 {
				idx := fieldPath[i+1 : i+end]
				parts = append(parts, idx)
				i += end
			}
		} else {
			current = append(current, r)
		}
	}
	if len(current) > 0 {
		parts = append(parts, string(current))
	}
	return parts
}

func parseDefaultValue(s string) interface{} {
	s = strings.Trim(s, " ")
	if s == "true" {
		return true
	}
	if s == "false" {
		return false
	}
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		return i
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f
	}
	return strings.Trim(s, `"`)
}

func (p *ExpressionParser) EvaluateExpression(expr string) (interface{}, error) {
	parsed, err := ParseExpression(expr)
	if err != nil {
		return nil, err
	}

	value := p.resolvePath(parsed.FieldPath, parsed.BasePath)
	hasValue := value != nil || (reflect.TypeOf(value) != nil && reflect.ValueOf(value).IsValid())

	for _, filter := range parsed.Filters {
		switch filter {
		case FilterDefault:
			if !hasValue {
				return parsed.DefaultVal, nil
			}
		case FilterRequired:
			if !hasValue {
				return nil, &ExpressionError{
					Code:       ErrCodeMissingField,
					Message:    "required field is missing",
					FieldPath:  parsed.FieldPath,
					Expression: expr,
				}
			}
		case FilterStrict:
			if !hasValue || value == nil {
				return nil, &ExpressionError{
					Code:        ErrCodeStrict,
					Message:    "strict mode: field is missing or nil",
					FieldPath:  parsed.FieldPath,
					Expression: expr,
				}
			}
		case FilterDefaultStrict:
			if !hasValue {
				return nil, &ExpressionError{
					Code:        ErrCodeStrict,
					Message:    "strict mode: field is missing",
					FieldPath:  parsed.FieldPath,
					Expression: expr,
				}
			}
		}
	}

	if !hasValue {
		if p.strict {
			return nil, &ExpressionError{
				Code:        ErrCodeStrict,
				Message:    "strict mode: field is missing",
				FieldPath:  parsed.FieldPath,
				Expression: expr,
			}
		}
		return nil, nil
	}

	return value, nil
}

func (p *ExpressionParser) resolvePath(fieldPath, rawPath string) interface{} {
	if p.data == nil {
		return nil
	}

	if strings.HasPrefix(rawPath, "literal:") {
		return rawPath[len("literal:"):]
	}

	if strings.HasPrefix(rawPath, "context.") {
		return p.resolveContextPath(rawPath)
	}

	v := reflect.ValueOf(p.data)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if v.Kind() == reflect.Map {
		result := p.resolveMapPath(v, fieldPath, rawPath)
		return result
	}

	if v.Kind() == reflect.Slice || v.Kind() == reflect.Array {
		result := p.resolveMapPath(v, fieldPath, rawPath)
		return result
	}

	parts := parseFieldPathParts(fieldPath)
	if len(parts) == 0 {
		return nil
	}

	return p.resolveStructOrMap(v, parts)
}

func (p *ExpressionParser) resolveContextPath(path string) interface{} {
	if p.execCtx == nil {
		return nil
	}

	field := strings.TrimPrefix(path, "context.")
	switch field {
	case "timestamp":
		return "now"
	case "user_id":
		if v, ok := p.execCtx.GetVariable("user_id"); ok {
			return v
		}
	case "session_id":
		if v, ok := p.execCtx.GetVariable("session_id"); ok {
			return v
		}
	case "pipeline_id":
		return p.execCtx.pipeline.ID
	}
	return nil
}

func (p *ExpressionParser) resolveMapPath(v reflect.Value, fieldPath, rawPath string) interface{} {
	// 解析带数组索引的路径，例如 messages[0].content
	parts := parseFieldPathParts(fieldPath)
	current := v

	for i, part := range parts {
		if current.Kind() != reflect.Map && current.Kind() != reflect.Struct && current.Kind() != reflect.Slice && current.Kind() != reflect.Array {
			return nil
		}

		switch current.Kind() {
		case reflect.Map:
			current = p.resolveMapKey(current, part, i)
		case reflect.Struct:
			current = p.resolveStructField(current, part, i)
		case reflect.Slice, reflect.Array:
			current = p.resolveArrayElement(current, part, i)
		}
		if current.IsValid() && current.Kind() == reflect.Interface {
			current = current.Elem()
		}
	}

	if !current.IsValid() {
		return nil
	}
	return current.Interface()
}

func (p *ExpressionParser) resolveMapKey(current reflect.Value, key string, index int) reflect.Value {
	k := reflect.ValueOf(key)
	val := current.MapIndex(k)
	if !val.IsValid() {
		num, err := strconv.Atoi(key)
		if err != nil {
			return reflect.Value{}
		}
		val = current.MapIndex(reflect.ValueOf(num))
	}
	if !val.IsValid() {
		return reflect.Value{}
	}
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return reflect.Value{}
		}
		return val.Elem()
	}
	return val
}

func (p *ExpressionParser) resolveStructField(current reflect.Value, fieldName string, index int) reflect.Value {
	for i := 0; i < current.Type().NumField(); i++ {
		field := current.Field(i)
		if field.Type().Name() == fieldName || current.Type().Field(i).Name == fieldName {
			if field.Kind() == reflect.Ptr {
				if field.IsNil() {
					return reflect.Value{}
				}
				return field.Elem()
			}
			return field
		}
	}
	return reflect.Value{}
}

func (p *ExpressionParser) resolveArrayElement(current reflect.Value, idx string, index int) reflect.Value {
	i, err := strconv.Atoi(idx)
	if err != nil {
		return reflect.Value{}
	}
	if i < 0 || i >= current.Len() {
		return reflect.Value{}
	}
	elem := current.Index(i)
	if elem.Kind() == reflect.Ptr {
		if elem.IsNil() {
			return reflect.Value{}
		}
		return elem.Elem()
	}
	return elem
}

func (p *ExpressionParser) resolveStructOrMap(v reflect.Value, parts []string) interface{} {
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	for i, part := range parts {
		switch v.Kind() {
		case reflect.Interface:
			if v.IsNil() {
				return nil
			}
			v = v.Elem()
		case reflect.Map:
			val := v.MapIndex(reflect.ValueOf(part))
			if !val.IsValid() {
				num, err := strconv.Atoi(part)
				if err != nil {
					return nil
				}
				val = v.MapIndex(reflect.ValueOf(num))
				if !val.IsValid() {
					return nil
				}
			}
			if !val.IsValid() {
				return nil
			}
			if val.Kind() == reflect.Ptr {
				if val.IsNil() {
					return nil
				}
				v = val.Elem()
			} else {
				v = val.Elem()
			}
		case reflect.Slice, reflect.Array:
			idx, err := strconv.Atoi(part)
			if err != nil {
				return nil
			}
			if idx < 0 || idx >= v.Len() {
				return nil
			}
			elem := v.Index(idx)
			if elem.Kind() == reflect.Ptr {
				if elem.IsNil() {
					return nil
				}
				v = elem.Elem()
			} else {
				v = elem
			}
		case reflect.Struct:
			field := v.FieldByName(part)
			if !field.IsValid() {
				return nil
			}
			if field.Kind() == reflect.Ptr {
				if field.IsNil() {
					return nil
				}
				v = field.Elem()
			} else {
				v = field
			}
		default:
			if i == len(parts)-1 {
				if v.IsValid() {
					return v.Interface()
				}
				return nil
			}
			return nil
		}
	}

	if !v.IsValid() {
		return nil
	}
	return v.Interface()
}

func (p *ExpressionParser) ResolveMap(data map[string]interface{}, expr string) (interface{}, error) {
	oldData := p.data
	p.data = data
	defer func() { p.data = oldData }()
	return p.EvaluateExpression(expr)
}

func ProcessNodeInputs(nodeInput *NodeInput, execCtx *ExecutionContext, inputs map[string]string, strict bool) (map[string]interface{}, error) {
	if len(inputs) == 0 {
		return nil, nil
	}

	result := make(map[string]interface{}, len(inputs))

	for key, expr := range inputs {
		if key == "" || expr == "" {
			continue
		}

		isDefault := false
		defaultVal := interface{}(nil)

		parsed, err := ParseExpression(expr)
		if err != nil {
			if !strict {
				continue
			}
			return nil, err
		}

		for _, filter := range parsed.Filters {
			if filter == FilterDefault {
				isDefault = true
				defaultVal = parsed.DefaultVal
			}
		}

		inputData := nodeInput.ToMap()
		parser := NewExpressionParserWithContext(inputData, execCtx)
		parser.SetStrict(strict)

		value, err := parser.EvaluateExpression(expr)
		if err != nil {
			if strict {
				return nil, err
			}
			if isDefault && defaultVal != nil {
				result[key] = defaultVal
				continue
			}
			continue
		}

		if value != nil || (isDefault && defaultVal != nil) {
			result[key] = value
		}
	}

	return result, nil
}

func (ni *NodeInput) ToMap() map[string]interface{} {
	if ni == nil {
		return nil
	}
	m := make(map[string]interface{})
	if ni.Content != "" {
		m["content"] = ni.Content
	}
	if len(ni.Messages) > 0 {
		m["messages"] = ni.Messages
	}
	if ni.Metadata != nil {
		for k, v := range ni.Metadata {
			m[k] = v
		}
	}
	if ni.Context != nil {
		for k, v := range ni.Context {
			m[k] = v
		}
	}
	return m
}

func (p *ExpressionParser) EvaluateMultiple(exprs []string) ([]interface{}, error) {
	results := make([]interface{}, 0, len(exprs))
	for _, expr := range exprs {
		val, err := p.EvaluateExpression(expr)
		if err != nil {
			return nil, err
		}
		results = append(results, val)
	}
	return results, nil
}

func IsExpression(expr string) bool {
	expr = strings.TrimSpace(expr)
	return strings.HasPrefix(expr, "{{") && strings.HasSuffix(expr, "}}")
}