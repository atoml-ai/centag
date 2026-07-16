//go:build protocol_openairesponses

package openairesponses

import "encoding/json"

// responsesRequest OpenAI Responses API 请求
type responsesRequest struct {
	Model        string           `json:"model"`
	Input        responsesInput   `json:"input"`
	Stream       bool             `json:"stream,omitempty"`
	Instructions string           `json:"instructions,omitempty"`
	Temperature  *float64         `json:"temperature,omitempty"`
	MaxTokens    *int             `json:"max_output_tokens,omitempty"`
	TopP         *float64         `json:"top_p,omitempty"`
}

// responsesInput 输入
type responsesInput struct {
	StringVal string        `json:"-"` // 当 input 是纯文本字符串时
	Items     []inputItem   `json:"-"` // 当 input 是数组时
}

func (ri *responsesInput) UnmarshalJSON(data []byte) error {
	if len(data) > 0 && data[0] == '"' {
		return ri.unmarshalString(data)
	}
	return ri.unmarshalArray(data)
}

func (ri *responsesInput) unmarshalString(data []byte) error {
	ri.StringVal = string(data[1 : len(data)-1])
	return nil
}

func (ri *responsesInput) unmarshalArray(data []byte) error {
	return json.Unmarshal(data, &ri.Items)
}

func (ri *responsesInput) MarshalJSON() ([]byte, error) {
	if ri.StringVal != "" {
		return json.Marshal(ri.StringVal)
	}
	return json.Marshal(ri.Items)
}

// inputItem 输入项
type inputItem struct {
	Type   string        `json:"type"`   // "message"
	Role   string        `json:"role"`   // "user", "assistant", "developer", "system"
	Content []contentPart `json:"content"` // 内容数组
}

// contentPart 内容部分
type contentPart struct {
	Type string `json:"type"` // "input_text"
	Text string `json:"text"`
}

// responsesResponse OpenAI Responses API 响应
type responsesResponse struct {
	ID      string          `json:"id"`
	Object  string          `json:"object"`
	Status  string          `json:"status"`
	Output  []outputItem    `json:"output"`
	Usage   *usageInfo      `json:"usage,omitempty"`
	Error   *responseError  `json:"error,omitempty"`
}

// outputItem 输出项
type outputItem struct {
	Type    string      `json:"type"`    // "message"
	ID      string      `json:"id"`
	Role    string      `json:"role"`    // "assistant"
	Content []contentPart `json:"content"`
	Status  string      `json:"status"`  // "completed", "in_progress"
}

// usageInfo 使用量信息
type usageInfo struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	TotalTokens              int `json:"total_tokens"`
	InputTokensDetails       *tokenDetails `json:"input_tokens_details,omitempty"`
	OutputTokensDetails      *tokenDetails `json:"output_tokens_details,omitempty"`
}

// tokenDetails token 详细信息
type tokenDetails struct {
	CachedTokens int `json:"cached_tokens,omitempty"`
}

// responseError 响应错误
type responseError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// sseEvent Responses SSE 事件
type sseEvent struct {
	Type     string      `json:"type"`      // "response.created", "response.output_item.added", "response.content_part.added", "response.output_text.delta", "response.completed", "error"
	Response *outputItem `json:"response,omitempty"`
	Item     *outputItem `json:"item,omitempty"`
	Part     *contentPart `json:"part,omitempty"`
	Delta    string      `json:"delta,omitempty"`
}
