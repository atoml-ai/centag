package pipeline

import (
	"context"
	"strings"
)

// RequestIDFromContext 从流水线执行上下文提取 request_id。
func RequestIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	execCtx, ok := ctx.Value(executionContextKey{}).(*ExecutionContext)
	if !ok || execCtx == nil {
		return ""
	}
	if id, ok := execCtx.GetVariable("request_id"); ok {
		if s, ok := id.(string); ok {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

// AppendRequestIDFields 在结构化日志字段末尾追加 request_id（若存在）。
func AppendRequestIDFields(ctx context.Context, fields ...interface{}) []interface{} {
	if id := RequestIDFromContext(ctx); id != "" {
		return append(fields, "request_id", id)
	}
	return fields
}
