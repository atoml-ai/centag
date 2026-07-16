package processor

import (
	"context"
	"strings"
	"unicode/utf8"
)

// ComplexityAnalyzerImpl 复杂度分析器实现
type ComplexityAnalyzerImpl struct {
	config *SplitConfig
}

// NewComplexityAnalyzer 创建复杂度分析器
func NewComplexityAnalyzer(config *SplitConfig) ComplexityAnalyzerImpl {
	return ComplexityAnalyzerImpl{config: config}
}

// Analyze 分析问题复杂度
func (a *ComplexityAnalyzerImpl) Analyze(ctx context.Context, question string) (*ComplexAnalysis, error) {
	analysis := &ComplexAnalysis{
		QuestionType: TypeSimple,
		SplitMarkers: []string{},
	}
	analysis.LengthScore = a.calculateLengthScore(question)
	structureScore, markers := a.analyzeStructure(question)
	analysis.StructureScore = structureScore
	analysis.SplitMarkers = markers
	analysis.QuestionType = a.classifyQuestionType(question, markers)
	analysis.ComplexityScore = a.calculateOverallComplexity(analysis)
	analysis.SuggestedSplitCount = a.suggestSplitCount(analysis)
	return analysis, nil
}

// ShouldSplit 根据分析结果判断是否需要拆分
func (a *ComplexityAnalyzerImpl) ShouldSplit(analysis *ComplexAnalysis, config *SplitConfig) bool {
	if !config.Enabled {
		return false
	}
	if analysis.QuestionType == TypeSimple {
		return false
	}
	if analysis.QuestionType == TypeCompound {
		return true
	}
	if analysis.ComplexityScore < config.ComplexityThreshold {
		return false
	}
	if analysis.LengthScore < 0.3 {
		return false
	}
	return true
}

func (a *ComplexityAnalyzerImpl) calculateLengthScore(question string) float32 {
	charCount := utf8.RuneCountInString(question)
	wordCount := len(strings.Fields(question))
	sentenceCount := a.countSentences(question)
	charScore := normalizeScore(float32(charCount), 0, 500)
	wordScore := normalizeScore(float32(wordCount), 0, 100)
	sentenceScore := normalizeScore(float32(sentenceCount), 0, 10)
	return (charScore + wordScore + sentenceScore) / 3
}

func (a *ComplexityAnalyzerImpl) analyzeStructure(question string) (float32, []string) {
	markerPatterns := []string{"?", "？", "。", "；", ";", "，", ",", "另外", "此外", "还有", "以及", "那", "那么", "接下来", "也", "或者"}
	foundMarkers := make([]string, 0)
	for _, marker := range markerPatterns {
		if strings.Contains(question, marker) {
			foundMarkers = append(foundMarkers, marker)
		}
	}
	markerCount := len(foundMarkers)
	structureScore := normalizeScore(float32(markerCount), 0, 5)
	return structureScore, foundMarkers
}

func (a *ComplexityAnalyzerImpl) classifyQuestionType(question string, markers []string) QuestionType {
	qMarkCount := strings.Count(question, "?") + strings.Count(question, "？")
	cnSuffixIndicators := []string{"是啥", "是什么", "有什么", "怎么样", "怎样", "如何", "为什么", "啥叫", "怎么", "是哪"}
	indicatorCount := 0
	for _, ind := range cnSuffixIndicators {
		indicatorCount += strings.Count(question, ind)
	}
	chainMarkers := []string{"那么", "接下来", "然后", "接着", "之后"}
	for _, marker := range chainMarkers {
		if strings.Contains(question, marker) {
			return TypeChain
		}
	}
	if qMarkCount > 1 || indicatorCount > 1 {
		return TypeCompound
	}
	if len(markers) > 3 || utf8.RuneCountInString(question) > 150 {
		return TypeComplex
	}
	return TypeSimple
}

func (a *ComplexityAnalyzerImpl) calculateOverallComplexity(analysis *ComplexAnalysis) float32 {
	weightLength := float32(0.4)
	weightStructure := float32(0.3)
	weightType := float32(0.3)
	typeScore := a.getTypeScore(analysis.QuestionType)
	overall := analysis.LengthScore*weightLength +
		analysis.StructureScore*weightStructure +
		typeScore*weightType
	return normalizeScore(overall, 0, 1)
}

func (a *ComplexityAnalyzerImpl) getTypeScore(qtype QuestionType) float32 {
	switch qtype {
	case TypeSimple:
		return 0.1
	case TypeCompound:
		return 0.7
	case TypeChain:
		return 0.8
	case TypeComplex:
		return 0.9
	default:
		return 0.5
	}
}

func (a *ComplexityAnalyzerImpl) suggestSplitCount(analysis *ComplexAnalysis) int {
	switch analysis.QuestionType {
	case TypeSimple:
		return 1
	case TypeCompound:
		questionCount := 0
		for _, marker := range analysis.SplitMarkers {
			if marker == "?" || marker == "？" {
				questionCount++
			}
		}
		return maxInt(questionCount, 2)
	case TypeChain:
		return 2
	case TypeComplex:
		if analysis.LengthScore > 0.7 {
			return 4
		}
		return 3
	default:
		return 2
	}
}

func (a *ComplexityAnalyzerImpl) countSentences(question string) int {
	sentenceEnders := []rune{'。', '！', '？', '.', '!', '?'}
	count := 0
	for _, char := range question {
		for _, ender := range sentenceEnders {
			if char == ender {
				count++
				break
			}
		}
	}
	return count
}

func normalizeScore(value, min, max float32) float32 {
	if max <= min {
		return 0
	}
	if value <= min {
		return 0
	}
	if value >= max {
		return 1
	}
	return (value - min) / (max - min)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
