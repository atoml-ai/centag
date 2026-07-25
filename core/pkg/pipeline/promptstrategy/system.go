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

	// 同步 raw_request_body：在原始 messages map 上改 system，保留 tool_calls / 多模态 content 等字段
	var rawBody []byte
	if len(in.RawBody) > 0 {
		synced, err := applySystemStrategyToRawBody(in.RawBody, in.Mode, gatewayPrompt, in.AppendPosition)
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

// SyncMessagesToRawBody 将结构化 Message 同步回 raw_request_body。
// 按索引合并：尽量保留原始 message 上的 tool_calls / 多模态 content 等扩展字段，
// 仅覆盖 role 与（当 content 为 string 时）content 文本。
func SyncMessagesToRawBody(rawBody []byte, messages []Message) ([]byte, error) {
	return syncMessagesToRawBody(rawBody, messages)
}

// ParseChatBody 从 JSON body 解析 messages 数组（content 仅取 string；多模态记为空串供规则检查）
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

// applySystemStrategyToRawBody 在原始 messages map 上应用 system 策略，保留非 system 消息的全部字段。
func applySystemStrategyToRawBody(rawBody []byte, mode SystemMode, gatewayPrompt string, pos AppendPosition) ([]byte, error) {
	if len(rawBody) == 0 {
		return rawBody, nil
	}
	var body map[string]interface{}
	if err := json.Unmarshal(rawBody, &body); err != nil {
		return nil, fmt.Errorf("unmarshal raw body: %w", err)
	}
	msgs, ok := body["messages"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("messages is not an array")
	}
	if pos == "" {
		pos = AppendPositionAfterClient
	}
	gateway := map[string]interface{}{
		"role":    "system",
		"content": gatewayPrompt,
	}
	var out []interface{}
	switch mode {
	case SystemModeReplace:
		out = make([]interface{}, 0, len(msgs)+1)
		out = append(out, gateway)
		for _, m := range msgs {
			mm, ok := m.(map[string]interface{})
			if !ok {
				continue
			}
			if role, _ := mm["role"].(string); strings.EqualFold(strings.TrimSpace(role), "system") {
				continue
			}
			out = append(out, mm)
		}
	case SystemModeAppend:
		out = appendSystemRawMessages(msgs, gateway, pos)
	default:
		return rawBody, nil
	}
	body["messages"] = out
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal raw body: %w", err)
	}
	return encoded, nil
}

func appendSystemRawMessages(msgs []interface{}, gateway map[string]interface{}, pos AppendPosition) []interface{} {
	switch pos {
	case AppendPositionBeforeClient:
		out := make([]interface{}, 0, len(msgs)+1)
		out = append(out, gateway)
		out = append(out, msgs...)
		return out
	case AppendPositionMergeLast:
		out := make([]interface{}, len(msgs))
		copy(out, msgs)
		lastSystemIdx := -1
		for i, m := range out {
			mm, ok := m.(map[string]interface{})
			if !ok {
				continue
			}
			if role, _ := mm["role"].(string); strings.EqualFold(strings.TrimSpace(role), "system") {
				lastSystemIdx = i
			}
		}
		if lastSystemIdx < 0 {
			return append([]interface{}{gateway}, out...)
		}
		mm := out[lastSystemIdx].(map[string]interface{})
		cloned := cloneJSONMap(mm)
		prev, _ := cloned["content"].(string)
		gw, _ := gateway["content"].(string)
		cloned["content"] = prev + "\n" + gw
		out[lastSystemIdx] = cloned
		return out
	default: // after_client
		out := make([]interface{}, 0, len(msgs)+1)
		lastSystemIdx := -1
		for i, m := range msgs {
			mm, ok := m.(map[string]interface{})
			if !ok {
				continue
			}
			if role, _ := mm["role"].(string); strings.EqualFold(strings.TrimSpace(role), "system") {
				lastSystemIdx = i
			}
		}
		if lastSystemIdx >= 0 {
			out = append(out, msgs[:lastSystemIdx+1]...)
			out = append(out, gateway)
			out = append(out, msgs[lastSystemIdx+1:]...)
			return out
		}
		out = append(out, gateway)
		out = append(out, msgs...)
		return out
	}
}

func cloneJSONMap(in map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// syncMessagesToRawBody 按索引合并结构化 Message 回 raw body，保留扩展字段。
func syncMessagesToRawBody(rawBody []byte, messages []Message) ([]byte, error) {
	if len(rawBody) == 0 {
		return rawBody, nil
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rawBody, &body); err != nil {
		return rawBody, fmt.Errorf("unmarshal raw body: %w", err)
	}

	origMsgs, _ := body["messages"].([]interface{})
	msgs := make([]interface{}, len(messages))
	for i, msg := range messages {
		var base map[string]interface{}
		if i < len(origMsgs) {
			if mm, ok := origMsgs[i].(map[string]interface{}); ok {
				base = cloneJSONMap(mm)
			}
		}
		if base == nil {
			base = map[string]interface{}{}
		}
		base["role"] = msg.Role
		// 仅当原 content 缺失或为 string 时覆盖，避免抹掉多模态 array content
		if _, isArr := base["content"].([]interface{}); !isArr {
			base["content"] = msg.Content
		} else if msg.Content != "" {
			base["content"] = msg.Content
		}
		msgs[i] = base
	}

	body["messages"] = msgs

	out, err := json.Marshal(body)
	if err != nil {
		return rawBody, fmt.Errorf("marshal raw body: %w", err)
	}

	return out, nil
}
