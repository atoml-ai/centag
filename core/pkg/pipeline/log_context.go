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

// OpencodeSessionFromContext 提取上游 OpenCode 路由/缓存所需的会话 ID。
// 取值顺序与代理路径 mode_dispatcher.attachTransparentRequestMetadata 对齐：
// metadata.opencode_session → session_id → "req_"+request_id；均无则返回空。
func OpencodeSessionFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	execCtx, ok := ctx.Value(executionContextKey{}).(*ExecutionContext)
	if !ok || execCtx == nil {
		return ""
	}
	if v, ok := execCtx.GetVariable("metadata"); ok {
		if meta, ok := v.(map[string]interface{}); ok {
			if s := stringMeta(meta, "opencode_session"); s != "" {
				return s
			}
		}
	}
	if v, ok := execCtx.GetVariable("session_id"); ok {
		if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	if rid := RequestIDFromContext(ctx); rid != "" {
		return "req_" + rid
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
