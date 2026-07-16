package pipeline

import "strings"

// InjectCacheControlFromMetadata copies per-request cache switches from pipeline
// metadata into ExecutionContext variables consumed by CacheNode.
func InjectCacheControlFromMetadata(execCtx *ExecutionContext, metadata map[string]interface{}) {
	if execCtx == nil || metadata == nil {
		return
	}
	for _, key := range []string{"cache_read", "cache_write", "cache_qa_split"} {
		if v, ok := metadata[key]; ok {
			execCtx.SetVariable(key, BoolFromInterface(v, true))
		}
	}
}

// BoolFromExecCtx reads a boolean execution-context variable.
func BoolFromExecCtx(execCtx *ExecutionContext, key string, defaultVal bool) bool {
	if execCtx == nil {
		return defaultVal
	}
	v, ok := execCtx.GetVariable(key)
	if !ok {
		return defaultVal
	}
	return BoolFromInterface(v, defaultVal)
}

// BoolFromInterface normalises metadata / context values to bool.
func BoolFromInterface(v interface{}, defaultVal bool) bool {
	switch x := v.(type) {
	case bool:
		return x
	case string:
		lower := strings.ToLower(strings.TrimSpace(x))
		switch lower {
		case "true", "enable", "1":
			return true
		case "false", "disable", "0":
			return false
		}
	}
	return defaultVal
}