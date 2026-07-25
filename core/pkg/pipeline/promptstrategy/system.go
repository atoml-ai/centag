package promptstrategy

import (
	"encoding/json"
	"fmt"
	"strings"
)

// SystemMode 定义 System Prompt 策略枚举
type SystemMode string

const (
	// SystemModePassthrough 透传客户端 system，不修改
	SystemModePassthrough SystemMode = "passthrough"
	// SystemModeAppend 保留客户端 system，追加网关文本
	SystemModeAppend SystemMode = "append"
	// SystemModeReplace 丢弃客户端 system，使用网关渲染后的文本
	SystemModeReplace SystemMode = "replace"
)

// IsValid 检查 SystemMode 是否有效
func (m SystemMode) IsValid() bool {
	switch m {
	case SystemModePassthrough, SystemModeAppend, SystemModeReplace:
		return true
	}
	return false
}

// AppendPosition 定义 append 模式下新 system 消息的插入位置
type AppendPosition string

const (
	// AppendPositionAfterClient 追加到客户端 system 之后（默认）
	AppendPositionAfterClient AppendPosition = "after_client"
	// AppendPositionBeforeClient 追加到客户端 system 之前
	AppendPositionBeforeClient AppendPosition = "before_client"
	// AppendPositionMergeLast 合并到最后一个 system 消息
	AppendPositionMergeLast AppendPosition = "merge_last"
)

// SystemApplyInput 定义 ApplySystemStrategy 的输入参数
type SystemApplyInput struct {
	// Mode 策略模式
	Mode SystemMode
	// GatewayPrompt 网关配置的 system_prompt（已渲染）
	GatewayPrompt string
	// AppendPosition append 模式下的插入位置（仅 append 模式有效）
	AppendPosition AppendPosition
	// Messages chat 消息列表
	Messages []Message
	// RawBody 原始请求体 JSON（可选，用于同步更新）
	RawBody []byte
}

// Message 对话消息
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// SystemApplyResult 定义 ApplySystemStrategy 的输出结果
type SystemApplyResult struct {
	// Messages 处理后的消息列表
	Messages []Message
	// RawBody 处理后的原始请求体（若入参有 RawBody 则同步写出）
	RawBody []byte
	// Applied 是否应用了策略
	Applied bool
	// Mode 实际使用的策略模式
	Mode SystemMode
}

// ApplySystemStrategy 应用 System Prompt 策略
// 支持 passthrough / append / replace 三种模式
func ApplySystemStrategy(in SystemApplyInput) (SystemApplyResult, error) {
	// 参数校验
	if !in.Mode.IsValid() {
		return SystemApplyResult{}, fmt.Errorf("invalid system mode: %s", in.Mode)
	}

	// passthrough 模式：不修改
	if in.Mode == SystemModePassthrough {
		return SystemApplyResult{
			Messages: in.Messages,
			RawBody:  in.RawBody,
			Applied:  false,
			Mode:     in.Mode,
		}, nil
	}

	// 处理空 gateway prompt：等价 passthrough
	gatewayPrompt := strings.TrimSpace(in.GatewayPrompt)
	if gatewayPrompt == "" {
		return SystemApplyResult{
			Messages: in.Messages,
			RawBody:  in.RawBody,
			Applied:  false,
			Mode:     SystemModePassthrough,
		}, nil
	}

	// 按模式处理消息
	var messages []Message
	switch in.Mode {
	case SystemModeReplace:
		messages = replaceSystemMessages(in.Messages, gatewayPrompt)
	case SystemModeAppend:
		pos := in.AppendPosition
		if pos == "" {
			pos = AppendPositionAfterClient
		}
		messages = appendSystemMessages(in.Messages, gatewayPrompt, pos)
	default:
		return SystemApplyResult{}, fmt.Errorf("unhandled system mode: %s", in.Mode)
	}

	// 同步 raw_request_body（若有）
	var rawBody []byte
	if len(in.RawBody) > 0 {
		synced, err := syncMessagesToRawBody(in.RawBody, messages)
		if err == nil {
			rawBody = synced
		} else {
			// 同步失败时透传原始 body，不阻断主路径
			rawBody = in.RawBody
		}
	} else {
		rawBody = in.RawBody
	}

	return SystemApplyResult{
		Messages: messages,
		RawBody:  rawBody,
		Applied:  true,
		Mode:     in.Mode,
	}, nil
}

// ResolveSystemMode 从兼容字段解析策略模式
// 优先使用 system_prompt_strategy，若未配置则从 inject_system_prompt 布尔映射
func ResolveSystemMode(strategy string, injectBool *bool) SystemMode {
	// 优先使用新字段
	if s := strings.TrimSpace(strategy); s != "" {
		mode := SystemMode(strings.ToLower(s))
		if mode.IsValid() {
			return mode
		}
	}

	// 兼容旧字段
	if injectBool != nil && *injectBool {
		return SystemModeReplace
	}

	// 默认透传
	return SystemModePassthrough
}

// replaceSystemMessages 替换所有 system 角色消息，仅保留一个网关 system
func replaceSystemMessages(messages []Message, gatewayPrompt string) []Message {
	result := make([]Message, 0, len(messages)+1)
	result = append(result, Message{
		Role:    "system",
		Content: gatewayPrompt,
	})
	for _, msg := range messages {
		if !strings.EqualFold(msg.Role, "system") {
			result = append(result, msg)
		}
	}
	return result
}

// appendSystemMessages 在客户端 system 消息之后/之前追加网关 system
func appendSystemMessages(messages []Message, gatewayPrompt string, pos AppendPosition) []Message {
	gatewayMsg := Message{
		Role:    "system",
		Content: gatewayPrompt,
	}

	switch pos {
	case AppendPositionBeforeClient:
		// 在所有 system 消息之前插入
		result := make([]Message, 0, len(messages)+1)
		result = append(result, gatewayMsg)
		result = append(result, messages...)
		return result

	case AppendPositionMergeLast:
		// 合并到最后一个 system 消息
		result := make([]Message, len(messages))
		copy(result, messages)
		lastSystemIdx := -1
		for i, msg := range result {
			if strings.EqualFold(msg.Role, "system") {
				lastSystemIdx = i
			}
		}
		if lastSystemIdx >= 0 {
			result[lastSystemIdx].Content = result[lastSystemIdx].Content + "\n" + gatewayPrompt
		} else {
			// 无 system 消息，作为第一条
			result = append([]Message{gatewayMsg}, result...)
		}
		return result

	default: // AppendPositionAfterClient
		// 在最后一个 system 消息之后插入
		result := make([]Message, 0, len(messages)+1)
		lastSystemIdx := -1
		for i, msg := range messages {
			if strings.EqualFold(msg.Role, "system") {
				lastSystemIdx = i
			}
		}
		if lastSystemIdx >= 0 {
			result = append(result, messages[:lastSystemIdx+1]...)
			result = append(result, gatewayMsg)
			result = append(result, messages[lastSystemIdx+1:]...)
		} else {
			// 无 system 消息，作为第一条
			result = append(result, gatewayMsg)
			result = append(result, messages...)
		}
		return result
	}
}

// SyncMessagesToRawBody 将消息列表同步到 raw_request_body
func SyncMessagesToRawBody(rawBody []byte, messages []Message) ([]byte, error) {
	return syncMessagesToRawBody(rawBody, messages)
}

// ParseChatBody 从 JSON body 解析 messages 数组
func ParseChatBody(body []byte) ([]Message, error) {
	if len(body) == 0 {
		return nil, fmt.Errorf("empty body")
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("unmarshal body: %w", err)
	}

	msgsRaw, ok := raw["messages"]
	if !ok {
		return nil, fmt.Errorf("no messages field in body")
	}

	msgsArr, ok := msgsRaw.([]interface{})
	if !ok {
		return nil, fmt.Errorf("messages is not an array")
	}

	messages := make([]Message, 0, len(msgsArr))
	for _, m := range msgsArr {
		mMap, ok := m.(map[string]interface{})
		if !ok {
			continue
		}
		role, _ := mMap["role"].(string)
		content, _ := mMap["content"].(string)
		messages = append(messages, Message{
			Role:    role,
			Content: content,
		})
	}

	return messages, nil
}

// syncMessagesToRawBody 将消息列表同步到 raw_request_body
func syncMessagesToRawBody(rawBody []byte, messages []Message) ([]byte, error) {
	if len(rawBody) == 0 {
		return rawBody, nil
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rawBody, &body); err != nil {
		return rawBody, fmt.Errorf("unmarshal raw body: %w", err)
	}

	// 转换消息为 JSON 兼容格式
	msgs := make([]interface{}, len(messages))
	for i, msg := range messages {
		m := map[string]interface{}{
			"role":    msg.Role,
			"content": msg.Content,
		}
		msgs[i] = m
	}

	body["messages"] = msgs

	out, err := json.Marshal(body)
	if err != nil {
		return rawBody, fmt.Errorf("marshal raw body: %w", err)
	}

	return out, nil
}
