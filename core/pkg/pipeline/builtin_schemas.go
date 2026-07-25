package pipeline

type NodeTypeSchemas struct {
	ConfigSchema  JSONSchema
	InputSchema   JSONSchema
	OutputSchema  JSONSchema
}

var BuiltinNodeSchemas = map[NodeType]NodeTypeSchemas{
	NodeTypeGenerator: {
		InputSchema: JSONSchema{
			"type": "object",
			"properties": map[string]interface{}{
				"content": map[string]interface{}{"type": "string"},
				"messages": map[string]interface{}{
					"type": "array",
				},
				"metadata": map[string]interface{}{"type": "object"},
			},
		},
		OutputSchema: JSONSchema{
			"type": "object",
			"properties": map[string]interface{}{
				"content":  map[string]interface{}{"type": "string"},
				"messages": map[string]interface{}{"type": "array"},
				"metadata": map[string]interface{}{"type": "object"},
				"passed":   map[string]interface{}{"type": "boolean"},
				"score":    map[string]interface{}{"type": "number"},
				"feedback": map[string]interface{}{"type": "string"},
			},
		},
		ConfigSchema: JSONSchema{
			"type": "object",
			"properties": map[string]interface{}{
				"backend":         map[string]interface{}{"type": "string"},
				"model":           map[string]interface{}{"type": "string"},
				"prompt_template": map[string]interface{}{"type": "string"},
				"system_prompt":   map[string]interface{}{"type": "string"},
				"temperature":     map[string]interface{}{"type": "number"},
				"max_tokens":      map[string]interface{}{"type": "integer"},
				"custom_config":   map[string]interface{}{"type": "object"},
				"template_vars":   map[string]interface{}{"type": "object"},
			},
		},
	},
	NodeTypeProcessor: {
		InputSchema: JSONSchema{
			"type": "object",
			"properties": map[string]interface{}{
				"content": map[string]interface{}{"type": "string"},
				"messages": map[string]interface{}{
					"type": "array",
				},
				"metadata": map[string]interface{}{"type": "object"},
			},
		},
		OutputSchema: JSONSchema{
			"type": "object",
			"properties": map[string]interface{}{
				"content":  map[string]interface{}{"type": "string"},
				"messages": map[string]interface{}{"type": "array"},
				"metadata": map[string]interface{}{"type": "object"},
				"passed":   map[string]interface{}{"type": "boolean"},
				"score":    map[string]interface{}{"type": "number"},
				"feedback": map[string]interface{}{"type": "string"},
			},
		},
		ConfigSchema: JSONSchema{
			"type": "object",
			"properties": map[string]interface{}{
				"backend":         map[string]interface{}{"type": "string"},
				"model":           map[string]interface{}{"type": "string"},
				"prompt_template": map[string]interface{}{"type": "string"},
				"system_prompt":   map[string]interface{}{"type": "string"},
				"temperature":     map[string]interface{}{"type": "number"},
				"max_tokens":      map[string]interface{}{"type": "integer"},
				"custom_config": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"operation":   map[string]interface{}{"type": "string"},
						"target_lang": map[string]interface{}{"type": "string"},
					},
				},
				"template_vars": map[string]interface{}{"type": "object"},
			},
		},
	},
	NodeTypeReviewer: {
		InputSchema: JSONSchema{
			"type": "object",
			"properties": map[string]interface{}{
				"content": map[string]interface{}{"type": "string"},
				"messages": map[string]interface{}{
					"type": "array",
				},
				"metadata": map[string]interface{}{"type": "object"},
			},
		},
		OutputSchema: JSONSchema{
			"type": "object",
			"properties": map[string]interface{}{
				"content":  map[string]interface{}{"type": "string"},
				"messages": map[string]interface{}{"type": "array"},
				"metadata": map[string]interface{}{"type": "object"},
				"passed":   map[string]interface{}{"type": "boolean"},
				"score":    map[string]interface{}{"type": "number"},
				"feedback": map[string]interface{}{"type": "string"},
			},
		},
		ConfigSchema: JSONSchema{
			"type": "object",
			"properties": map[string]interface{}{
				"backend":         map[string]interface{}{"type": "string"},
				"model":           map[string]interface{}{"type": "string"},
				"prompt_template": map[string]interface{}{"type": "string"},
				"system_prompt":   map[string]interface{}{"type": "string"},
				"temperature":     map[string]interface{}{"type": "number"},
				"max_tokens":      map[string]interface{}{"type": "integer"},
				"custom_config":   map[string]interface{}{"type": "object"},
				"template_vars":   map[string]interface{}{"type": "object"},
			},
		},
	},
	NodeTypeRouter: {
		InputSchema: JSONSchema{
			"type": "object",
			"properties": map[string]interface{}{
				"content": map[string]interface{}{"type": "string"},
				"messages": map[string]interface{}{
					"type": "array",
				},
				"metadata": map[string]interface{}{"type": "object"},
			},
		},
		OutputSchema: JSONSchema{
			"type": "object",
			"properties": map[string]interface{}{
				"content":  map[string]interface{}{"type": "string"},
				"messages": map[string]interface{}{
					"type": "array",
				},
				"metadata": map[string]interface{}{"type": "object"},
				"passed":   map[string]interface{}{"type": "boolean"},
				"score":    map[string]interface{}{"type": "number"},
				"feedback": map[string]interface{}{"type": "string"},
			},
		},
		ConfigSchema: JSONSchema{
			"type": "object",
			"properties": map[string]interface{}{
				"backend":         map[string]interface{}{"type": "string"},
				"model":           map[string]interface{}{"type": "string"},
				"prompt_template": map[string]interface{}{"type": "string"},
				"system_prompt":   map[string]interface{}{"type": "string"},
				"temperature":     map[string]interface{}{"type": "number"},
				"max_tokens":      map[string]interface{}{"type": "integer"},
				"custom_config": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"routing_strategy": map[string]interface{}{
							"type": "string",
							"enum": []string{
								"keyword_contains",
								"keyword_prefix",
								"ordered",
								"regex_only",
								"llm_classify",
							},
							"description": "路由匹配策略。llm_classify 通过 LLM 语义分类，准确率高但增加一次 LLM 调用。",
						},
						"default_route": map[string]interface{}{"type": "string"},
						"routes":        map[string]interface{}{"type": "object"},
						"route_rules":   map[string]interface{}{"type": "array"},
						"classify_prompt": map[string]interface{}{
							"type":        "string",
							"description": "llm_classify 策略下可选的自定义分类 Prompt（留空使用内置默认）。可用变量：{{.input}}",
						},
					},
				},
				"template_vars": map[string]interface{}{"type": "object"},
			},
		},
	},
	NodeTypeAggregator: {
		InputSchema: JSONSchema{
			"type": "object",
			"properties": map[string]interface{}{
				"content": map[string]interface{}{"type": "string"},
				"messages": map[string]interface{}{
					"type": "array",
				},
				"metadata": map[string]interface{}{"type": "object"},
			},
		},
		OutputSchema: JSONSchema{
			"type": "object",
			"properties": map[string]interface{}{
				"content":  map[string]interface{}{"type": "string"},
				"messages": map[string]interface{}{"type": "array"},
				"metadata": map[string]interface{}{"type": "object"},
				"passed":   map[string]interface{}{"type": "boolean"},
				"score":    map[string]interface{}{"type": "number"},
				"feedback": map[string]interface{}{"type": "string"},
			},
		},
		ConfigSchema: JSONSchema{
			"type": "object",
			"properties": map[string]interface{}{
				"backend":         map[string]interface{}{"type": "string"},
				"model":           map[string]interface{}{"type": "string"},
				"prompt_template": map[string]interface{}{"type": "string"},
				"system_prompt":   map[string]interface{}{"type": "string"},
				"temperature":     map[string]interface{}{"type": "number"},
				"max_tokens":      map[string]interface{}{"type": "integer"},
				"custom_config": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"strategy": map[string]interface{}{
							"type": "string",
							"enum": []string{"concat", "merge", "summarize", "vote", "best"},
						},
					},
				},
				"template_vars": map[string]interface{}{"type": "object"},
			},
		},
	},
	NodeTypeMemory: {
		InputSchema: JSONSchema{
			"type": "object",
			"properties": map[string]interface{}{
				"content":  map[string]interface{}{"type": "string"},
				"context":  map[string]interface{}{"type": "object"},
				"metadata": map[string]interface{}{"type": "object"},
			},
		},
		OutputSchema: JSONSchema{
			"type": "object",
			"properties": map[string]interface{}{
				"content":  map[string]interface{}{"type": "string"},
				"metadata": map[string]interface{}{"type": "object"},
			},
		},
		ConfigSchema: JSONSchema{
			"type": "object",
			"properties": map[string]interface{}{
				"backend": map[string]interface{}{"type": "string"},
				"custom_config": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"query_type": map[string]interface{}{
							"type":        "string",
							"enum":        []string{"user", "session", "global"},
							"description": "Query type: user/session/global",
						},
						"top_k": map[string]interface{}{
							"type":        "integer",
							"description": "Number of results to return",
						},
						"filter": map[string]interface{}{
							"type":        "object",
							"description": "Filter conditions",
						},
					},
				},
			},
		},
	},
	NodeTypeAudit: {
		InputSchema: JSONSchema{
			"type": "object",
			"properties": map[string]interface{}{
				"content":  map[string]interface{}{"type": "string"},
				"metadata": map[string]interface{}{"type": "object"},
			},
		},
		OutputSchema: JSONSchema{
			"type": "object",
			"properties": map[string]interface{}{
				"content":   map[string]interface{}{"type": "string"},
				"metadata":  map[string]interface{}{"type": "object"},
				"passed":    map[string]interface{}{"type": "boolean"},
				"score":     map[string]interface{}{"type": "number"},
				"feedback":  map[string]interface{}{"type": "string"},
			},
		},
		ConfigSchema: JSONSchema{
			"type": "object",
			"properties": map[string]interface{}{
				"backend":         map[string]interface{}{"type": "string"},
				"model":           map[string]interface{}{"type": "string"},
				"prompt_template": map[string]interface{}{"type": "string"},
				"custom_config": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"rules": map[string]interface{}{
							"type":        "array",
							"description": "Audit rules",
						},
						"threshold": map[string]interface{}{
							"type":        "number",
							"description": "Pass threshold (0-1)",
						},
					},
				},
			},
		},
	},
	NodeTypeOptimize: {
		InputSchema: JSONSchema{
			"type": "object",
			"properties": map[string]interface{}{
				"content":  map[string]interface{}{"type": "string"},
				"metadata": map[string]interface{}{"type": "object"},
			},
		},
		OutputSchema: JSONSchema{
			"type": "object",
			"properties": map[string]interface{}{
				"content":  map[string]interface{}{"type": "string"},
				"metadata": map[string]interface{}{"type": "object"},
			},
		},
		ConfigSchema: JSONSchema{
			"type": "object",
			"properties": map[string]interface{}{
				"backend":         map[string]interface{}{"type": "string"},
				"model":           map[string]interface{}{"type": "string"},
				"prompt_template": map[string]interface{}{"type": "string"},
				"custom_config": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"strategy": map[string]interface{}{
							"type":        "string",
							"enum":        []string{"clarity", "structure", "completeness"},
							"description": "Optimization strategy",
						},
					},
				},
			},
		},
	},
	// Phase 4: Cache Node Schema
	NodeTypeCache: {
		InputSchema: JSONSchema{
			"type": "object",
			"properties": map[string]interface{}{
				"content":  map[string]interface{}{"type": "string"},
				"messages": map[string]interface{}{"type": "array"},
				"metadata": map[string]interface{}{"type": "object"},
			},
		},
		OutputSchema: JSONSchema{
			"type": "object",
			"properties": map[string]interface{}{
				"content":  map[string]interface{}{"type": "string"},
				"metadata": map[string]interface{}{"type": "object"},
			},
		},
		ConfigSchema: JSONSchema{
			"type": "object",
			"properties": map[string]interface{}{
				"custom_config": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"operation": map[string]interface{}{
							"type":        "string",
							"enum":        []string{"read", "write", "delete"},
							"description": "Cache operation: read/write/delete",
						},
						"strategy": map[string]interface{}{
							"type":        "string",
							"enum":        []string{"exact", "semantic", "hybrid"},
							"description": "Cache strategy",
						},
						"storage_type": map[string]interface{}{
							"type":        "string",
							"enum":        []string{"memory", "redis", "sqlite"},
							"description": "Storage backend type",
						},
						"ttl": map[string]interface{}{
							"type":        "integer",
							"description": "Cache TTL in seconds",
						},
						"key_template": map[string]interface{}{
							"type":        "string",
							"description": "Cache key template, supports {{model}} and {{hash}}",
						},
						"config": map[string]interface{}{
							"type":        "object",
							"description": "Storage-specific configuration",
						},
					},
				},
			},
		},
	},
	// Phase 4: Token Usage Node Schema
	NodeTypeTokenUsage: {
		InputSchema: JSONSchema{
			"type": "object",
			"properties": map[string]interface{}{
				"content":  map[string]interface{}{"type": "string"},
				"messages": map[string]interface{}{"type": "array"},
				"metadata": map[string]interface{}{"type": "object"},
			},
		},
		OutputSchema: JSONSchema{
			"type": "object",
			"properties": map[string]interface{}{
				"content":  map[string]interface{}{"type": "string"},
				"metadata": map[string]interface{}{"type": "object"},
			},
		},
		ConfigSchema: JSONSchema{
			"type": "object",
			"properties": map[string]interface{}{
				"custom_config": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"operation": map[string]interface{}{
							"type":        "string",
							"enum":        []string{"record", "query", "aggregate"},
							"description": "Token usage operation: record/query/aggregate",
						},
						"storage_type": map[string]interface{}{
							"type":        "string",
							"enum":        []string{"memory", "sqlite", "postgresql"},
							"description": "Storage backend type",
						},
						"record_fields": map[string]interface{}{
							"type":        "array",
							"items":       map[string]interface{}{"type": "string"},
							"description": "Fields to record",
						},
						"config": map[string]interface{}{
							"type":        "object",
							"description": "Storage-specific configuration",
						},
					},
				},
			},
		},
	},
	NodeTypeScheduler: {
		InputSchema: JSONSchema{
			"type": "object",
			"properties": map[string]interface{}{
				"content":  map[string]interface{}{"type": "string"},
				"metadata": map[string]interface{}{"type": "object"},
			},
		},
		OutputSchema: JSONSchema{
			"type": "object",
			"properties": map[string]interface{}{
				"content": map[string]interface{}{"type": "string"},
				"metadata": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"scheduler_decision":   map[string]interface{}{"type": "boolean"},
						"backend_id":           map[string]interface{}{"type": "string"},
						"model":                map[string]interface{}{"type": "string"},
						"reason":               map[string]interface{}{"type": "string"},
						"task_type":            map[string]interface{}{"type": "string"},
						"estimated_cost":       map[string]interface{}{"type": "number"},
						"estimated_latency_ms": map[string]interface{}{"type": "integer"},
					},
				},
			},
		},
		ConfigSchema: JSONSchema{
			"type": "object",
			"properties": map[string]interface{}{
				"model": map[string]interface{}{"type": "string"},
				"custom_config": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"strategy": map[string]interface{}{
							"type":        "string",
							"description": "Scheduling strategy: balance, cost, quality, latency",
						},
					},
				},
			},
		},
	},
	NodeTypeTransparentForward: {
		InputSchema: JSONSchema{
			"type": "object",
			"properties": map[string]interface{}{
				"content":  map[string]interface{}{"type": "string"},
				"metadata": map[string]interface{}{"type": "object"},
			},
		},
		OutputSchema: JSONSchema{
			"type": "object",
			"properties": map[string]interface{}{
				"content": map[string]interface{}{"type": "string"},
				"metadata": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"raw_passthrough": map[string]interface{}{"type": "boolean"},
						"target_url":      map[string]interface{}{"type": "string"},
						"status_code":     map[string]interface{}{"type": "integer"},
					},
				},
			},
		},
		ConfigSchema: JSONSchema{
			"type": "object",
			"properties": map[string]interface{}{
				"backend": map[string]interface{}{"type": "string"},
				"custom_config": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"default_scheme": map[string]interface{}{"type": "string", "default": "https"},
					},
				},
			},
		},
	},
	NodeTypeToolCallInjector: {
		InputSchema: JSONSchema{
			"type": "object",
			"properties": map[string]interface{}{
				"content":  map[string]interface{}{"type": "string"},
				"metadata": map[string]interface{}{"type": "object"},
			},
		},
		OutputSchema: JSONSchema{
			"type": "object",
			"properties": map[string]interface{}{
				"content": map[string]interface{}{"type": "string"},
				"metadata": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"node_type":      map[string]interface{}{"type": "string"},
						"injected_count": map[string]interface{}{"type": "integer"},
						"tool_call_ids": map[string]interface{}{
							"type":  "array",
							"items": map[string]interface{}{"type": "string"},
						},
					},
				},
				"tool_calls": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"id":   map[string]interface{}{"type": "string"},
							"type": map[string]interface{}{"type": "string"},
							"function": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"name":      map[string]interface{}{"type": "string"},
									"arguments": map[string]interface{}{"type": "string"},
								},
							},
						},
					},
				},
			},
		},
		ConfigSchema: JSONSchema{
			"type": "object",
			"properties": map[string]interface{}{
				"custom_config": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"tool_calls": map[string]interface{}{
							"type": "array",
							"items": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"id":   map[string]interface{}{"type": "string"},
									"type": map[string]interface{}{"type": "string"},
									"function": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"name":      map[string]interface{}{"type": "string"},
											"arguments": map[string]interface{}{"type": "string"},
										},
									},
								},
							},
						},
						"condition": map[string]interface{}{"type": "string"},
					},
				},
			},
		},
	},
	NodeTypeUserPromptOps: {
		InputSchema: JSONSchema{
			"type": "object",
			"properties": map[string]interface{}{
				"content":  map[string]interface{}{"type": "string"},
				"messages": map[string]interface{}{"type": "array"},
				"metadata": map[string]interface{}{"type": "object"},
			},
		},
		OutputSchema: JSONSchema{
			"type": "object",
			"properties": map[string]interface{}{
				"content":  map[string]interface{}{"type": "string"},
				"messages": map[string]interface{}{"type": "array"},
				"metadata": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"node_type": map[string]interface{}{"type": "string"},
						"action":    map[string]interface{}{"type": "string"},
					},
				},
			},
		},
		ConfigSchema: JSONSchema{
			"type": "object",
			"properties": map[string]interface{}{
				"custom_config": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"check": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"enabled":        map[string]interface{}{"type": "boolean"},
								"deny_patterns":  map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
								"on_hit":         map[string]interface{}{"type": "string", "enum": []interface{}{"log", "redact", "block"}},
							},
						},
						"optimize": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"enabled":             map[string]interface{}{"type": "boolean"},
								"max_user_chars":      map[string]interface{}{"type": "integer"},
								"collapse_whitespace": map[string]interface{}{"type": "boolean"},
							},
						},
					},
				},
			},
		},
	},
	NodeTypeOutputPostOps: {
		InputSchema: JSONSchema{
			"type": "object",
			"properties": map[string]interface{}{
				"content":  map[string]interface{}{"type": "string"},
				"metadata": map[string]interface{}{"type": "object"},
			},
		},
		OutputSchema: JSONSchema{
			"type": "object",
			"properties": map[string]interface{}{
				"content": map[string]interface{}{"type": "string"},
				"metadata": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"node_type": map[string]interface{}{"type": "string"},
						"ops_applied": map[string]interface{}{
							"type":  "array",
							"items": map[string]interface{}{"type": "string"},
						},
					},
				},
			},
		},
		ConfigSchema: JSONSchema{
			"type": "object",
			"properties": map[string]interface{}{
				"custom_config": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"ops": map[string]interface{}{
							"type": "array",
							"items": map[string]interface{}{
								"type": "string",
								"enum": []interface{}{"trim_space", "strip_markdown_fence", "extract_json", "json_compact"},
							},
						},
						"on_invalid_json": map[string]interface{}{
							"type": "string",
							"enum": []interface{}{"pass", "wrap_error_object"},
						},
						"stream_mode": map[string]interface{}{
							"type": "string",
							"enum": []interface{}{"skip", "buffer"},
						},
						"max_buffer_bytes": map[string]interface{}{
							"type": "integer",
						},
					},
				},
			},
		},
	},
}

func GetBuiltinSchemas(nodeType NodeType) NodeTypeSchemas {
	return BuiltinNodeSchemas[nodeType]
}