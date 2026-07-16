package processor

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"

	"centag/core/internal/llm"
	"centag/core/pkg/logger"
	"centag/core/pkg/utils"

	"go.uber.org/zap"
)

// QASplitter 问答拆分器
// 使用小模型对问答对进行语义拆分
type QASplitter struct {
	chatService llm.ChatService
	prompt      string
	enabled     bool
}

// QASplitterConfig 拆分器配置
type QASplitterConfig struct {
	ChatService llm.ChatService
	Prompt      string
	Enabled     bool
}

// NewQASplitter 创建问答拆分器
func NewQASplitter(config *QASplitterConfig) *QASplitter {
	if config == nil {
		return &QASplitter{
			enabled: false,
		}
	}

	return &QASplitter{
		chatService: config.ChatService,
		prompt:      config.Prompt,
		enabled:     config.Enabled,
	}
}

// SplitQA 拆分问答对
func (qs *QASplitter) SplitQA(ctx context.Context, question, answer string) (*QASplitResult, error) {
	// 检查是否启用
	if !qs.enabled || qs.chatService == nil {
		if logger.Logger != nil {
			logger.Debug("QASplitter not enabled, returning original QA")
		}
		return &QASplitResult{
			Split: false,
			QAPairs: []QAPair{
				{
					Question: question,
					Answer:   answer,
				},
			},
		}, nil
	}

	// 构建完整提示
	fullPrompt := qs.buildPrompt(question, answer)

	// 调用小模型进行拆分
	request := &llm.ChatRequest{
		Messages: []llm.ChatMessage{
			{
				Role:    "user",
				Content: llm.NewMessageContent(fullPrompt),
			},
		},
		Temperature: 0.3, // 较低温度以获得更稳定的输出
		MaxTokens:   2000,
		// 不使用Format: "json",让模型自由输出,避免小模型不兼容
	}

	logger.Info("Calling chat model for QA splitting",
		zap.String("question", utils.TruncateString(question, 100)),
		zap.String("model", qs.chatService.GetProviderInfo().Model))

	response, err := qs.chatService.Chat(ctx, request)
	if err != nil {
		logger.Warn("Failed to call chat model for QA splitting, returning original QA",
			zap.Error(err))
		// 拆分失败，返回原始问答对
		return &QASplitResult{
			Split: false,
			QAPairs: []QAPair{
				{
					Question: question,
					Answer:   answer,
				},
			},
		}, nil
	}

	logger.Info("QA split model response",
		zap.String("response", utils.TruncateString(response.Content, 500)))

	// 解析响应 - 先尝试直接JSON解析
	result := &QASplitResult{}
	if err := json.Unmarshal([]byte(response.Content), result); err != nil {
		// JSON解析失败,尝试提取JSON部分
		logger.Warn("Failed to parse chat response as JSON, trying to extract JSON",
			zap.Error(err))

		// 尝试从响应中提取JSON块(可能在```json...```中)
		jsonContent := extractJSON(response.Content)
		if jsonContent != "" {
			if err := json.Unmarshal([]byte(jsonContent), result); err != nil {
				logger.Warn("Failed to parse extracted JSON, returning original QA",
					zap.Error(err),
					zap.String("extracted_json", jsonContent))
				return &QASplitResult{
					Split: false,
					QAPairs: []QAPair{{Question: question, Answer: answer}},
				}, nil
			}
		} else {
			// 完全无法解析,返回原始问答对
			return &QASplitResult{
				Split: false,
				QAPairs: []QAPair{{Question: question, Answer: answer}},
			}, nil
		}
	}

	// 验证结果
	if len(result.QAPairs) == 0 {
		logger.Warn("No QA pairs returned, using original QA")
		return &QASplitResult{
			Split: false,
			QAPairs: []QAPair{
				{
					Question: question,
					Answer:   answer,
				},
			},
		}, nil
	}

	// 判断是否真的拆分了: 检查是否返回了多个QA对,或者QA对的内容与原始不同
	actualSplit := len(result.QAPairs) > 1 ||
		(len(result.QAPairs) == 1 &&
			(result.QAPairs[0].Question != question || result.QAPairs[0].Answer != answer))

	if !actualSplit {
		logger.Info("QA splitting not needed by model")
		// 返回原始问答对
		return &QASplitResult{
			Split: false,
			QAPairs: []QAPair{
				{
					Question: question,
					Answer:   answer,
				},
			},
		}, nil
	}

	// 使用实际拆分的结果,更新split标志
	result.Split = actualSplit

	logger.Info("QA splitting completed",
		zap.Int("qa_pairs_count", len(result.QAPairs)),
		zap.Bool("split", result.Split))

	return result, nil
}

// buildPrompt 构建完整提示
func (qs *QASplitter) buildPrompt(question, answer string) string {
	// 替换模板中的占位符
	prompt := strings.ReplaceAll(qs.prompt, "{{question}}", question)
	prompt = strings.ReplaceAll(prompt, "{{answer}}", answer)

	return prompt
}

// SetEnabled 设置是否启用
func (qs *QASplitter) SetEnabled(enabled bool) {
	qs.enabled = enabled
}

// SetChatService 设置对话服务
func (qs *QASplitter) SetChatService(service llm.ChatService) {
	qs.chatService = service
}

// IsEnabled 检查是否启用
func (qs *QASplitter) IsEnabled() bool {
	return qs.enabled
}

// GetPrompt 获取提示词
func (qs *QASplitter) GetPrompt() string {
	return qs.prompt
}

// SetPrompt 设置提示词
func (qs *QASplitter) SetPrompt(prompt string) {
	qs.prompt = prompt
}

// GetChatService 获取对话服务
func (qs *QASplitter) GetChatService() llm.ChatService {
	return qs.chatService
}

// extractJSON 从字符串中提取JSON内容
func extractJSON(s string) string {
	// 尝试匹配 ```json ... ``` (使用非贪婪匹配)
	jsonRegex := regexp.MustCompile("(?s)```json\\s*(.+?)\\s*```")
	matches := jsonRegex.FindStringSubmatch(s)
	if len(matches) > 1 {
		return cleanJSON(matches[1])
	}
	
	// 尝试匹配 ``` ... ``` (没有 json 标记)
	codeBlockRegex := regexp.MustCompile("(?s)```\\s*(.+?)\\s*```")
	matches = codeBlockRegex.FindStringSubmatch(s)
	if len(matches) > 1 {
		content := strings.TrimSpace(matches[1])
		// 检查是否以 { 开头（可能是 JSON）
		if strings.HasPrefix(content, "{") {
			return cleanJSON(content)
		}
	}

	// 尝试匹配 { ... } (第一个完整的JSON对象)
	startIndex := strings.Index(s, "{")
	if startIndex == -1 {
		return ""
	}

	// 找到匹配的闭合括号
	braceCount := 0
	for i := startIndex; i < len(s); i++ {
		if s[i] == '{' {
			braceCount++
		} else if s[i] == '}' {
			braceCount--
			if braceCount == 0 {
				jsonStr := s[startIndex : i+1]
				return cleanJSON(jsonStr)
			}
		}
	}

	return ""
}

// cleanJSON 清理JSON字符串中的常见语法错误
func cleanJSON(s string) string {
	// 移除对象或数组最后一个元素后的多余逗号
	re := regexp.MustCompile(`,\s*([}\]])`)
	s = re.ReplaceAllString(s, "$1")

	// 尝试直接解析
	var temp interface{}
	if err := json.Unmarshal([]byte(s), &temp); err == nil {
		if cleaned, err := json.Marshal(temp); err == nil {
			return string(cleaned)
		}
	}

	// 解析失败，尝试修复字符串值中未转义的双引号，然后再解析一次
	repaired := repairUnescapedQuotes(s)
	if err := json.Unmarshal([]byte(repaired), &temp); err == nil {
		if cleaned, err := json.Marshal(temp); err == nil {
			return string(cleaned)
		}
	}

	return s
}

// repairUnescapedQuotes 修复 JSON 字符串值中未转义的双引号。
// 模型常常在答案中输出 "Golang" 这样含原始引号的文本，导致 JSON 非法。
// 策略：逐字符扫描，遇到字符串内部的 " 时，通过向后查找下一个非空白字符来
// 判断该引号是否真正结束了字符串（终止符为 : , } ]），否则将其转义。
func repairUnescapedQuotes(s string) string {
	var buf strings.Builder
	buf.Grow(len(s) + 16)
	inString := false
	n := len(s)
	i := 0

	for i < n {
		c := s[i]

		// 处理已有的转义序列，直接透传
		if inString && c == '\\' && i+1 < n {
			buf.WriteByte(c)
			buf.WriteByte(s[i+1])
			i += 2
			continue
		}

		if c == '"' {
			if !inString {
				inString = true
				buf.WriteByte(c)
			} else {
				// 向后查找下一个非空白字符，判断此引号是否为字符串终止符
				j := i + 1
				for j < n && (s[j] == ' ' || s[j] == '\t' || s[j] == '\r' || s[j] == '\n') {
					j++
				}
				var next byte
				if j < n {
					next = s[j]
				}
				// 字符串结束后只会跟 : , } ] 或到达末尾
				if j >= n || next == ':' || next == ',' || next == '}' || next == ']' {
					inString = false
					buf.WriteByte(c)
				} else {
					// 内部未转义引号，转义后继续
					buf.WriteString(`\"`)
				}
			}
		} else {
			buf.WriteByte(c)
		}
		i++
	}

	return buf.String()
}


