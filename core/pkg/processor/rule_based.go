package processor

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// RuleBasedSplitter 基于规则的问题拆分器
// 使用预定义的规则和模式来识别和拆分复杂问题
// 这是一个极简实现，不依赖 internal 包，供插件系统外的基础回退使用。
type RuleBasedSplitter struct {
	config   *SplitConfig
	analyzer ComplexityAnalyzerImpl
	strategy SplitStrategy
}

// NewRuleBasedSplitter 创建基于规则的拆分器
func NewRuleBasedSplitter(config *SplitConfig) (*RuleBasedSplitter, error) {
	if config == nil {
		config = DefaultSplitConfig()
	}
	if len(config.SplitMarkers) == 0 {
		config.SplitMarkers = []string{"?", "？", "。", "；", ";", "，", ",", "另外", "此外", "还有", "以及"}
	}
	if config.MaxSplitCount <= 0 {
		config.MaxSplitCount = 10
	}
	if config.MinSplitLength <= 0 {
		config.MinSplitLength = 10
	}

	analyzer := NewComplexityAnalyzer(config)
	splitter := &RuleBasedSplitter{
		config:   config,
		analyzer: analyzer,
		strategy: StrategyRuleBased,
	}
	return splitter, nil
}

// Split 拆分问题
func (s *RuleBasedSplitter) Split(ctx context.Context, question string) ([]*SubQuestion, error) {
	shouldSplit, analysis, err := s.ShouldSplit(ctx, question)
	if err != nil {
		return nil, fmt.Errorf("complexity analysis failed: %w", err)
	}
	if !shouldSplit {
		return []*SubQuestion{{ID: generateQuestionID(0), Content: question, Order: 0}}, nil
	}

	var subQuestions []*SubQuestion
	switch analysis.QuestionType {
	case TypeCompound:
		subQuestions = s.splitCompoundQuestion(question)
	case TypeChain:
		subQuestions = s.splitChainQuestion(question)
	case TypeComplex:
		subQuestions = s.splitComplexQuestion(question)
	default:
		subQuestions = []*SubQuestion{{ID: generateQuestionID(0), Content: question, Order: 0}}
	}

	if len(subQuestions) > s.config.MaxSplitCount {
		subQuestions = subQuestions[:s.config.MaxSplitCount]
	}
	return subQuestions, nil
}

// ShouldSplit 判断是否需要拆分
func (s *RuleBasedSplitter) ShouldSplit(ctx context.Context, question string) (bool, *ComplexAnalysis, error) {
	if !s.config.Enabled {
		return false, &ComplexAnalysis{}, nil
	}
	analysis, err := s.analyzer.Analyze(ctx, question)
	if err != nil {
		return false, nil, err
	}
	shouldSplit := s.analyzer.ShouldSplit(analysis, s.config)
	return shouldSplit, analysis, nil
}

// GetStrategy 获取拆分策略
func (s *RuleBasedSplitter) GetStrategy() SplitStrategy {
	return s.strategy
}

func (s *RuleBasedSplitter) splitCompoundQuestion(question string) []*SubQuestion {
	subQuestions := make([]*SubQuestion, 0)
	parts := s.splitByMarkers(question, []string{"?", "？"})
	if len(parts) <= 1 {
		parts = s.splitByMarkers(question, []string{"，", ","})
	}
	for i, part := range parts {
		trimmed := strings.TrimSpace(part)
		trimmed = strings.TrimRight(trimmed, "，,")
		trimmed = strings.TrimSpace(trimmed)
		if len(trimmed) < s.config.MinSplitLength {
			continue
		}
		if !strings.HasSuffix(trimmed, "?") && !strings.HasSuffix(trimmed, "？") {
			trimmed += "？"
		}
		subQuestions = append(subQuestions, &SubQuestion{ID: generateQuestionID(i), Content: trimmed, Order: i})
	}
	return subQuestions
}

func (s *RuleBasedSplitter) splitChainQuestion(question string) []*SubQuestion {
	subQuestions := make([]*SubQuestion, 0)
	chainMarkers := []string{"那么", "接下来", "然后", "接着", "之后"}
	parts := []string{question}
	for _, marker := range chainMarkers {
		if strings.Contains(question, marker) {
			parts = s.splitByMarkers(question, []string{marker})
			break
		}
	}
	if len(parts) == 1 {
		parts = s.splitByMarkers(question, s.config.SplitMarkers)
	}
	for i, part := range parts {
		trimmed := strings.TrimSpace(part)
		if len(trimmed) < s.config.MinSplitLength {
			continue
		}
		dependencies := []string{}
		if i > 0 {
			dependencies = append(dependencies, generateQuestionID(i-1))
		}
		subQuestions = append(subQuestions, &SubQuestion{ID: generateQuestionID(i), Content: trimmed, Order: i, Dependencies: dependencies})
	}
	return subQuestions
}

func (s *RuleBasedSplitter) splitComplexQuestion(question string) []*SubQuestion {
	subQuestions := make([]*SubQuestion, 0)
	parts := s.splitByMultipleMarkers(question)
	for i, part := range parts {
		trimmed := strings.TrimSpace(part)
		if len(trimmed) < s.config.MinSplitLength {
			continue
		}
		if utf8.RuneCountInString(trimmed) > 100 {
			sentences := s.splitBySentence(trimmed)
			for j, sentence := range sentences {
				if strings.TrimSpace(sentence) == "" {
					continue
				}
				subQuestions = append(subQuestions, &SubQuestion{ID: fmt.Sprintf("%d-%d", i, j), Content: strings.TrimSpace(sentence), Order: len(subQuestions)})
			}
		} else {
			subQuestions = append(subQuestions, &SubQuestion{ID: generateQuestionID(i), Content: trimmed, Order: i})
		}
	}
	return subQuestions
}

func (s *RuleBasedSplitter) splitByMarkers(text string, markers []string) []string {
	parts := []string{text}
	for _, marker := range markers {
		newParts := make([]string, 0)
		for _, part := range parts {
			splitParts := strings.Split(part, marker)
			for i, splitPart := range splitParts {
				trimmed := strings.TrimSpace(splitPart)
				if trimmed == "" {
					continue
				}
				if i < len(splitParts)-1 {
					trimmed += marker
				}
				newParts = append(newParts, trimmed)
			}
		}
		parts = newParts
	}
	return parts
}

func (s *RuleBasedSplitter) splitByMultipleMarkers(text string) []string {
	if len(s.config.SplitMarkers) == 0 {
		return []string{text}
	}
	escapedMarkers := make([]string, len(s.config.SplitMarkers))
	for i, marker := range s.config.SplitMarkers {
		escapedMarkers[i] = regexp.QuoteMeta(marker)
	}
	pattern := strings.Join(escapedMarkers, "|")
	re := regexp.MustCompile(pattern)
	parts := re.Split(text, -1)
	result := make([]string, 0)
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func (s *RuleBasedSplitter) splitBySentence(text string) []string {
	sentenceEnders := []string{"。", "！", "？", ".", "!", "?"}
	parts := []string{text}
	for _, ender := range sentenceEnders {
		newParts := make([]string, 0)
		for _, part := range parts {
			splitParts := strings.Split(part, ender)
			for i, splitPart := range splitParts {
				trimmed := strings.TrimSpace(splitPart)
				if trimmed == "" {
					continue
				}
				if i < len(splitParts)-1 {
					trimmed += ender
				}
				newParts = append(newParts, trimmed)
			}
		}
		parts = newParts
	}
	return parts
}

func generateQuestionID(index int) string {
	return fmt.Sprintf("q-%d", index)
}
