package processor

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Synthesizer 答案合成器实现
// 支持多种合成策略，不依赖 internal 包
type Synthesizer struct {
	config     *SynthesisConfig
	strategy   SynthesisStrategy
	llmService LLMService // 可选，用于 LLM 合成策略
}

// NewSynthesizer 创建答案合成器
func NewSynthesizer(config *SynthesisConfig) (*Synthesizer, error) {
	return NewSynthesizerWithLLM(config, nil)
}

// NewSynthesizerWithLLM 创建答案合成器（带 LLM 服务）
func NewSynthesizerWithLLM(config *SynthesisConfig, llmService LLMService) (*Synthesizer, error) {
	if config.Strategy == "" {
		config.Strategy = SynthesisStrategyTemplate
	}
	if config.Template == "" {
		config.Template = getDefaultTemplate(config.Strategy)
	}
	if config.MaxRetry <= 0 {
		config.MaxRetry = 3
	}
	return &Synthesizer{
		config:     config,
		strategy:   config.Strategy,
		llmService: llmService,
	}, nil
}

// Synthesize 合成答案
func (s *Synthesizer) Synthesize(ctx context.Context, originalQuestion string, subAnswers []*SubAnswer) (string, error) {
	if len(subAnswers) == 0 {
		return "", fmt.Errorf("no sub-answers to synthesize")
	}
	if len(subAnswers) == 1 {
		return subAnswers[0].Answer, nil
	}

	var result string
	var err error

	switch s.strategy {
	case SynthesisStrategyConcat:
		result = s.synthesizeByConcat(subAnswers)
	case SynthesisStrategyTemplate:
		result = s.synthesizeByTemplate(originalQuestion, subAnswers)
	case SynthesisStrategyLLM:
		result, err = s.synthesizeByLLM(ctx, originalQuestion, subAnswers)
		if err != nil {
			result = s.synthesizeByTemplate(originalQuestion, subAnswers)
		}
	case SynthesisStrategyHybrid:
		result = s.synthesizeByHybrid(originalQuestion, subAnswers)
	default:
		result = s.synthesizeByTemplate(originalQuestion, subAnswers)
	}

	return result, nil
}

// GetStrategy 获取合成策略
func (s *Synthesizer) GetStrategy() SynthesisStrategy {
	return s.strategy
}

// ValidateSubAnswers 验证子答案
func (s *Synthesizer) ValidateSubAnswers(subAnswers []*SubAnswer) error {
	if len(subAnswers) == 0 {
		return fmt.Errorf("no sub-answers provided")
	}
	for i, answer := range subAnswers {
		if strings.TrimSpace(answer.Answer) == "" {
			return fmt.Errorf("sub-answer %d is empty", i)
		}
	}
	return nil
}

func (s *Synthesizer) synthesizeByConcat(subAnswers []*SubAnswer) string {
	var builder strings.Builder
	if s.config.PreserveOrder {
		sort.Slice(subAnswers, func(i, j int) bool {
			return subAnswers[i].QuestionID < subAnswers[j].QuestionID
		})
	}
	for i, answer := range subAnswers {
		if i > 0 {
			builder.WriteString("\n\n")
		}
		if s.config.EnableCitation {
			builder.WriteString(fmt.Sprintf("[问题%d] %s\n", i+1, answer.Question))
			builder.WriteString(fmt.Sprintf("答案: %s", answer.Answer))
		} else {
			builder.WriteString(answer.Answer)
		}
	}
	return builder.String()
}

func (s *Synthesizer) synthesizeByTemplate(originalQuestion string, subAnswers []*SubAnswer) string {
	template := s.config.Template
	if s.config.PreserveOrder {
		sort.Slice(subAnswers, func(i, j int) bool {
			return subAnswers[i].QuestionID < subAnswers[j].QuestionID
		})
	}
	answersPart := s.buildAnswersPart(subAnswers)
	result := template
	result = strings.ReplaceAll(result, "{{original_question}}", originalQuestion)
	result = strings.ReplaceAll(result, "{{question_count}}", fmt.Sprintf("%d", len(subAnswers)))
	result = strings.ReplaceAll(result, "{{answers}}", answersPart)
	return result
}

func (s *Synthesizer) synthesizeByLLM(ctx context.Context, originalQuestion string, subAnswers []*SubAnswer) (string, error) {
	if s.llmService == nil {
		return s.synthesizeByTemplate(originalQuestion, subAnswers), nil
	}
	if err := s.ValidateSubAnswers(subAnswers); err != nil {
		return "", fmt.Errorf("invalid sub-answers: %w", err)
	}
	prompt := s.buildSynthesisPrompt(originalQuestion, subAnswers)
	var lastErr error
	for attempt := 0; attempt < s.config.MaxRetry; attempt++ {
		if attempt > 0 {
			waitTime := s.calculateBackoff(attempt)
			select {
			case <-time.After(waitTime):
			case <-ctx.Done():
				return "", ctx.Err()
			}
		}
		result, genErr := s.llmService.Generate(ctx, prompt)
		if genErr == nil && s.validateSynthesisResult(result) {
			return result, nil
		}
		lastErr = genErr
		if lastErr == nil {
			lastErr = fmt.Errorf("invalid synthesis result")
		}
	}
	return s.synthesizeByTemplate(originalQuestion, subAnswers), nil
}

func (s *Synthesizer) calculateBackoff(attempt int) time.Duration {
	backoff := time.Duration(1<<uint(attempt-1)) * time.Second
	if backoff > 10*time.Second {
		backoff = 10 * time.Second
	}
	return backoff
}

func (s *Synthesizer) validateSynthesisResult(result string) bool {
	if result == "" {
		return false
	}
	trimmed := strings.TrimSpace(result)
	if len(trimmed) < 10 {
		return false
	}
	errorKeywords := []string{"无法回答", "cannot answer", "作为 AI 语言模型", "as an AI language model", "抱歉，我无法", "sorry, i cannot"}
	lowerResult := strings.ToLower(trimmed)
	for _, keyword := range errorKeywords {
		if strings.Contains(lowerResult, strings.ToLower(keyword)) {
			return false
		}
	}
	return true
}

func (s *Synthesizer) buildSynthesisPrompt(originalQuestion string, subAnswers []*SubAnswer) string {
	var answersPart strings.Builder
	for i, answer := range subAnswers {
		answersPart.WriteString(fmt.Sprintf("子问题 %d: %s\n答案: %s\n\n", i+1, answer.Question, answer.Answer))
	}
	return fmt.Sprintf(`请根据以下子问题的答案，为原始问题生成一个完整、自然、连贯的回答。

原始问题：%s

%s

要求：
1. 直接回答原始问题，不要重复子问题
2. 将各个子答案自然地融合在一起
3. 保持逻辑连贯和条理清晰
4. 语言简洁明了，避免冗余
5. 如果子答案之间有矛盾，请指出并给出你的判断

请生成最终答案：`, originalQuestion, answersPart.String())
}

func (s *Synthesizer) synthesizeByHybrid(originalQuestion string, subAnswers []*SubAnswer) string {
	totalLength := 0
	for _, answer := range subAnswers {
		totalLength += len(answer.Answer)
	}
	avgLength := totalLength / len(subAnswers)
	if avgLength < 100 {
		return s.synthesizeByConcat(subAnswers)
	}
	return s.synthesizeByTemplate(originalQuestion, subAnswers)
}

func (s *Synthesizer) buildAnswersPart(subAnswers []*SubAnswer) string {
	var builder strings.Builder
	for i, answer := range subAnswers {
		if i > 0 {
			builder.WriteString("\n\n")
		}
		if s.config.EnableCitation {
			cacheMark := ""
			if answer.FromCache {
				cacheMark = " (来自缓存)"
			}
			builder.WriteString(fmt.Sprintf("%d. %s%s\n   %s", i+1, answer.Question, cacheMark, answer.Answer))
		} else {
			builder.WriteString(answer.Answer)
		}
	}
	return builder.String()
}

func getDefaultTemplate(strategy SynthesisStrategy) string {
	switch strategy {
	case SynthesisStrategyTemplate, SynthesisStrategyHybrid:
		return `针对您的问题："{{original_question}}"

以下是相关解答：

{{answers}}

希望以上回答对您有帮助！如果还有其他问题，请随时提问。`
	case SynthesisStrategyConcat:
		return `{{answers}}`
	default:
		return `{{original_question}}

{{answers}}`
	}
}
