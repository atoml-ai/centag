//go:build protocol_gemini

package gemini

// geminiRequest Gemini API 请求
type geminiRequest struct {
	Contents          []content         `json:"contents"`
	SystemInstruction *content          `json:"systemInstruction,omitempty"`
	GenerationConfig  *generationConfig `json:"generationConfig,omitempty"`
}

// content 内容
type content struct {
	Role  string `json:"role"`
	Parts []part `json:"parts"`
}

// part 部分
type part struct {
	Text string `json:"text,omitempty"`
}

// generationConfig 生成配置
type generationConfig struct {
	Temperature     *float64 `json:"temperature,omitempty"`
	MaxOutputTokens *int     `json:"maxOutputTokens,omitempty"`
	TopP            *float64 `json:"topP,omitempty"`
}

// geminiResponse Gemini API 响应
type geminiResponse struct {
	Candidates    []candidate    `json:"candidates"`
	UsageMetadata *usageMetadata `json:"usageMetadata,omitempty"`
}

// candidate 候选
type candidate struct {
	Index        int     `json:"index,omitempty"`
	Content      content `json:"content"`
	FinishReason string  `json:"finishReason,omitempty"`
}

// usageMetadata 使用元数据
type usageMetadata struct {
	PromptTokenCount     int `json:"promptTokenCount,omitempty"`
	CandidatesTokenCount int `json:"candidatesTokenCount,omitempty"`
	TotalTokenCount      int `json:"totalTokenCount,omitempty"`
}

// geminiStreamChunk Gemini 流式响应块
type geminiStreamChunk struct {
	Candidates    []candidate    `json:"candidates"`
	UsageMetadata *usageMetadata `json:"usageMetadata,omitempty"`
}
