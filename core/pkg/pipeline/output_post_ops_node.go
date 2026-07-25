package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// OutputPostOpsConfig 定义 output_post_ops 节点的配置
type OutputPostOpsConfig struct {
	// Ops 操作列表
	Ops []string `json:"ops"`
	// OnInvalidJSON 无效 JSON 处置: pass | wrap_error_object
	OnInvalidJSON string `json:"on_invalid_json,omitempty"`
	// StreamMode 流式模式: skip | buffer
	StreamMode string `json:"stream_mode,omitempty"`
	// LLM Phase B 占位（Phase A 忽略）
	LLM *LLMPlaceholderConfig `json:"llm,omitempty"`
}

// OutputPostOpsNode 输出后处理节点 - 字符串规范化
type OutputPostOpsNode struct {
	BaseNode
	opsConfig OutputPostOpsConfig
}

// NewOutputPostOpsNode 创建输出后处理节点
func NewOutputPostOpsNode(config NodeConfig) (PipelineNode, error) {
	node := &OutputPostOpsNode{
		BaseNode: BaseNode{
			config:      config,
			timeout:     30,
			retryConfig: DefaultRetryConfig(),
		},
	}

	// 解析配置
	if config.CustomConfig != nil {
		configBytes, err := json.Marshal(config.CustomConfig)
		if err != nil {
			return nil, fmt.Errorf("output_post_ops node: marshal custom_config: %w", err)
		}
		if err := json.Unmarshal(configBytes, &node.opsConfig); err != nil {
			return nil, fmt.Errorf("output_post_ops node: unmarshal config: %w", err)
		}
	}

	// 默认流式模式为 skip
	if node.opsConfig.StreamMode == "" {
		node.opsConfig.StreamMode = "skip"
	}

	return node, nil
}

// Type 返回节点类型
func (n *OutputPostOpsNode) Type() NodeType {
	return NodeTypeOutputPostOps
}

// Validate 验证节点配置
func (n *OutputPostOpsNode) Validate() error {
	return nil
}

// Execute 执行输出后处理
func (n *OutputPostOpsNode) Execute(ctx context.Context, input *NodeInput) (*NodeOutput, error) {
	log := LoggerFromContext(ctx)

	content := input.Content
	if content == "" {
		return &NodeOutput{Content: content}, nil
	}

	// 应用操作链
	result := n.applyOps(content, log)

	return &NodeOutput{
		Content:  result,
		Metadata: map[string]interface{}{"node_type": "output_post_ops"},
	}, nil
}

// applyOps 应用操作链
func (n *OutputPostOpsNode) applyOps(content string, log Logger) string {
	for _, op := range n.opsConfig.Ops {
		switch op {
		case "trim_space":
			content = strings.TrimSpace(content)
		case "strip_markdown_fence":
			content = stripMarkdownFence(content)
		case "extract_json":
			extracted, ok := extractJSON(content)
			if ok {
				content = extracted
			} else if n.opsConfig.OnInvalidJSON == "wrap_error_object" {
				content = fmt.Sprintf(`{"error": "invalid json", "original": %s}`, escapeJSONString(content))
			}
		case "json_compact":
			compacted, err := compactJSON(content)
			if err == nil {
				content = compacted
			}
		default:
			if log != nil {
				log.Warn("[OutputPostOpsNode] unknown op", "op", op)
			}
		}
	}
	return content
}

// stripMarkdownFence 移除 Markdown 代码围栏
func stripMarkdownFence(content string) string {
	content = strings.TrimSpace(content)

	// 移除 ```json ... ``` 或 ``` ... ```
	if strings.HasPrefix(content, "```") {
		lines := strings.Split(content, "\n")
		if len(lines) >= 2 {
			// 移除第一行（``` 或 ```json）
			start := 1
			// 移除最后一行（```）
			end := len(lines)
			if strings.TrimSpace(lines[end-1]) == "```" {
				end--
			}
			content = strings.Join(lines[start:end], "\n")
		}
	}

	return strings.TrimSpace(content)
}

// extractJSON 从文本中提取 JSON 对象
func extractJSON(content string) (string, bool) {
	content = strings.TrimSpace(content)

	// 尝试直接解析
	var js interface{}
	if err := json.Unmarshal([]byte(content), &js); err == nil {
		return content, true
	}

	// 查找 { ... } 或 [ ... ]
	start := -1
	end := -1

	for i, ch := range content {
		if ch == '{' || ch == '[' {
			if start == -1 {
				start = i
			}
		}
		if ch == '}' || ch == ']' {
			end = i + 1
		}
	}

	if start >= 0 && end > start {
		candidate := content[start:end]
		var js interface{}
		if err := json.Unmarshal([]byte(candidate), &js); err == nil {
			return candidate, true
		}
	}

	return content, false
}

// compactJSON 压缩 JSON（移除空白）
func compactJSON(content string) (string, error) {
	var js interface{}
	if err := json.Unmarshal([]byte(content), &js); err != nil {
		return content, err
	}

	compacted, err := json.Marshal(js)
	if err != nil {
		return content, err
	}

	return string(compacted), nil
}

// escapeJSONString 转义字符串用于 JSON
func escapeJSONString(s string) string {
	b, _ := json.Marshal(s)
	// 移除外层引号
	result := string(b)
	if len(result) >= 2 && result[0] == '"' && result[len(result)-1] == '"' {
		result = result[1 : len(result)-1]
	}
	return result
}
