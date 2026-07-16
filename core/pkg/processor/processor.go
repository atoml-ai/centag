package processor

import (
	"context"
	"fmt"
)

// QuestionProcessorImpl 问题处理器实现
type QuestionProcessorImpl struct {
	splitter   QuestionSplitter
	synthesizer AnswerSynthesizer
	config     *ProcessorConfig
	llmService LLMService
}

// NewQuestionProcessor 创建问题处理器（不带 LLM 服务）
func NewQuestionProcessor(config *ProcessorConfig) (*QuestionProcessorImpl, error) {
	return NewQuestionProcessorWithLLM(config, nil)
}

// NewQuestionProcessorWithLLM 创建问题处理器（支持 LLM 服务）
func NewQuestionProcessorWithLLM(config *ProcessorConfig, llmService LLMService) (*QuestionProcessorImpl, error) {
	if config == nil {
		config = DefaultProcessorConfig()
	}

	// 创建拆分器
	splitter, err := NewRuleBasedSplitter(&config.Split)
	if err != nil {
		return nil, fmt.Errorf("failed to create splitter: %w", err)
	}

	// 创建合成器
	synthesizer, err := NewSynthesizerWithLLM(&config.Synthesis, llmService)
	if err != nil {
		return nil, fmt.Errorf("failed to create synthesizer: %w", err)
	}

	return &QuestionProcessorImpl{
		splitter:    splitter,
		synthesizer: synthesizer,
		config:      config,
		llmService:  llmService,
	}, nil
}

// SplitQuestion 拆分复杂问题为多个子问题
func (p *QuestionProcessorImpl) SplitQuestion(ctx context.Context, question string) (*SplitResult, error) {
	shouldSplit, analysis, err := p.splitter.ShouldSplit(ctx, question)
	if err != nil {
		return nil, fmt.Errorf("complexity analysis failed: %w", err)
	}

	result := &SplitResult{
		OriginalQuestion: question,
		ShouldSplit:      shouldSplit,
		Complexity:       analysis,
		Strategy:         p.splitter.GetStrategy(),
	}

	if !shouldSplit {
		result.SubQuestions = []*SubQuestion{
			{ID: "q-0", Content: question, Order: 0},
		}
		return result, nil
	}

	subQuestions, err := p.splitter.Split(ctx, question)
	if err != nil {
		return nil, fmt.Errorf("split failed: %w", err)
	}

	result.SubQuestions = subQuestions
	return result, nil
}

// SynthesizeAnswer 合成多个子答案为一个完整答案
func (p *QuestionProcessorImpl) SynthesizeAnswer(ctx context.Context, originalQuestion string, subAnswers []*SubAnswer) (string, error) {
	if len(subAnswers) == 0 {
		return "", fmt.Errorf("no sub-answers to synthesize")
	}
	return p.synthesizer.Synthesize(ctx, originalQuestion, subAnswers)
}

// GetStrategy 获取当前使用的策略
func (p *QuestionProcessorImpl) GetStrategy() SplitStrategy {
	return p.splitter.GetStrategy()
}

// SetStrategy 设置拆分策略
func (p *QuestionProcessorImpl) SetStrategy(strategy SplitStrategy) error {
	// 重新创建拆分器
	config := p.config.Split
	config.Strategy = strategy
	splitter, err := NewRuleBasedSplitter(&config)
	if err != nil {
		return fmt.Errorf("failed to create splitter with new strategy: %w", err)
	}
	p.splitter = splitter
	return nil
}

// GetSplitter 获取底层拆分器
func (p *QuestionProcessorImpl) GetSplitter() QuestionSplitter {
	return p.splitter
}
