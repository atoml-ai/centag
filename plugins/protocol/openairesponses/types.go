//go:build protocol_openairesponses

package openairesponses

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// responsesRequest OpenAI Responses API 请求
type responsesRequest struct {
	Model        string         `json:"model"`
	Input        responsesInput `json:"input"`
	Stream       bool           `json:"stream,omitempty"`
	Instructions string         `json:"instructions,omitempty"`
	Temperature  *float64       `json:"temperature,omitempty"`
	MaxTokens    *int           `json:"max_output_tokens,omitempty"`
	TopP         *float64       `json:"top_p,omitempty"`
}

// responsesInput 输入：string | message 数组
type responsesInput struct {
	StringVal string      `json:"-"`
	Items     []inputItem `json:"-"`
}

func (ri *responsesInput) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || string(data) == "null" {
		return nil
	}
	switch data[0] {
	case '"':
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		ri.StringVal = s
		return nil
	case '[':
		return json.Unmarshal(data, &ri.Items)
	default:
		return fmt.Errorf("responses input: expected string or array, got %s", summarizeJSON(data))
	}
}

func (ri responsesInput) MarshalJSON() ([]byte, error) {
	if ri.StringVal != "" {
		return json.Marshal(ri.StringVal)
	}
	return json.Marshal(ri.Items)
}

// inputItem 输入项（OpenAI / OpenCode Zen 等）
type inputItem struct {
	Type    string         `json:"type"` // "message" (optional)
	Role    string         `json:"role"` // "user", "assistant", "developer", "system"
	Content flexibleContent `json:"content"`
}

// flexibleContent accepts content as a plain string or []contentPart.
// OpenCode Zen sends string; OpenAI docs often use part arrays.
type flexibleContent struct {
	Text  string
	Parts []contentPart
}

func (fc *flexibleContent) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || string(data) == "null" {
		return nil
	}
	switch data[0] {
	case '"':
		return json.Unmarshal(data, &fc.Text)
	case '[':
		return json.Unmarshal(data, &fc.Parts)
	default:
		return fmt.Errorf("content: expected string or array, got %s", summarizeJSON(data))
	}
}

func (fc flexibleContent) MarshalJSON() ([]byte, error) {
	if fc.Text != "" {
		return json.Marshal(fc.Text)
	}
	return json.Marshal(fc.Parts)
}

func (fc flexibleContent) PlainText() string {
	if strings.TrimSpace(fc.Text) != "" {
		return fc.Text
	}
	var b strings.Builder
	for _, cp := range fc.Parts {
		switch cp.Type {
		case "input_text", "output_text", "text", "":
			b.WriteString(cp.Text)
		}
	}
	return b.String()
}

// contentPart 内容部分
type contentPart struct {
	Type string `json:"type"` // "input_text" / "output_text"
	Text string `json:"text"`
}

// responsesResponse OpenAI Responses API 响应
type responsesResponse struct {
	ID     string         `json:"id"`
	Object string         `json:"object"`
	Status string         `json:"status"`
	Output []outputItem   `json:"output"`
	Usage  *usageInfo     `json:"usage,omitempty"`
	Error  *responseError `json:"error,omitempty"`
}

// outputItem 输出项
type outputItem struct {
	Type    string        `json:"type"`
	ID      string        `json:"id"`
	Role    string        `json:"role"`
	Content []contentPart `json:"content"`
	Status  string        `json:"status"`
}

// usageInfo 使用量信息
type usageInfo struct {
	InputTokens         int           `json:"input_tokens"`
	OutputTokens        int           `json:"output_tokens"`
	TotalTokens         int           `json:"total_tokens"`
	InputTokensDetails  *tokenDetails `json:"input_tokens_details,omitempty"`
	OutputTokensDetails *tokenDetails `json:"output_tokens_details,omitempty"`
}

type tokenDetails struct {
	CachedTokens int `json:"cached_tokens,omitempty"`
}

type responseError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type sseEvent struct {
	Type     string       `json:"type"`
	Response *outputItem  `json:"response,omitempty"`
	Item     *outputItem  `json:"item,omitempty"`
	Part     *contentPart `json:"part,omitempty"`
	Delta    string       `json:"delta,omitempty"`
}

func summarizeJSON(data []byte) string {
	if len(data) > 48 {
		return string(data[:48]) + "…"
	}
	return string(data)
}
