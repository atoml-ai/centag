//go:build backend_azure

package azure

// openaiRequest OpenAI 兼容请求
type openaiRequest struct {
	Model            string    `json:"model"`
	Messages         []message `json:"messages"`
	Temperature      float64   `json:"temperature,omitempty"`
	MaxTokens        int       `json:"max_tokens,omitempty"`
	TopP             float64   `json:"top_p,omitempty"`
	FrequencyPenalty float64   `json:"frequency_penalty,omitempty"`
	PresencePenalty  float64   `json:"presence_penalty,omitempty"`
	Stop             []string  `json:"stop,omitempty"`
	Stream           bool      `json:"stream,omitempty"`
}

// message 消息
type message struct {
	Role             string `json:"role"`
	Content          string `json:"content"`
	ReasoningContent string `json:"reasoning_content,omitempty"`
}

// openaiResponse OpenAI 兼容响应
type openaiResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []choice `json:"choices"`
	Usage   usage    `json:"usage"`
}

// choice 选择
type choice struct {
	Index        int     `json:"index"`
	Message      message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

// usage 使用情况
type usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// openaiStreamChunk OpenAI 流式响应块
type openaiStreamChunk struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []streamChoice `json:"choices"`
}

// streamChoice 流式选择
type streamChoice struct {
	Index        int          `json:"index"`
	Delta        streamDelta  `json:"delta"`
	FinishReason string       `json:"finish_reason"`
}

// streamDelta 流式增量
type streamDelta struct {
	Role             string `json:"role,omitempty"`
	Content          string `json:"content,omitempty"`
	ReasoningContent string `json:"reasoning_content,omitempty"`
}
