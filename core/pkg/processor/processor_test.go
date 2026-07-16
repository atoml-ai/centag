package processor

import (
	"context"
	"testing"
)

func TestDefaultSplitConfig(t *testing.T) {
	cfg := DefaultSplitConfig()
	if cfg == nil {
		t.Fatal("DefaultSplitConfig() returned nil")
	}
	if !cfg.Enabled {
		t.Error("expected Enabled to be true")
	}
	if cfg.Strategy != StrategyRuleBased {
		t.Errorf("expected StrategyRuleBased, got %s", cfg.Strategy)
	}
	if cfg.ComplexityThreshold != 0.5 {
		t.Errorf("expected ComplexityThreshold 0.5, got %f", cfg.ComplexityThreshold)
	}
	if len(cfg.SplitMarkers) == 0 {
		t.Error("expected non-empty SplitMarkers")
	}
}

func TestDefaultSynthesisConfig(t *testing.T) {
	cfg := DefaultSynthesisConfig()
	if cfg == nil {
		t.Fatal("DefaultSynthesisConfig() returned nil")
	}
	if cfg.Strategy != SynthesisStrategyTemplate {
		t.Errorf("expected SynthesisStrategyTemplate, got %s", cfg.Strategy)
	}
	if cfg.MaxRetry != 3 {
		t.Errorf("expected MaxRetry 3, got %d", cfg.MaxRetry)
	}
}

func TestDefaultProcessorConfig(t *testing.T) {
	cfg := DefaultProcessorConfig()
	if cfg == nil {
		t.Fatal("DefaultProcessorConfig() returned nil")
	}
	if cfg.Split.Strategy != StrategyRuleBased {
		t.Error("expected split strategy to be StrategyRuleBased")
	}
	if cfg.Synthesis.Strategy != SynthesisStrategyTemplate {
		t.Error("expected synthesis strategy to be SynthesisStrategyTemplate")
	}
}

func TestNewRuleBasedSplitter(t *testing.T) {
	cfg := DefaultSplitConfig()
	splitter, err := NewRuleBasedSplitter(cfg)
	if err != nil {
		t.Fatalf("NewRuleBasedSplitter failed: %v", err)
	}
	if splitter == nil {
		t.Fatal("NewRuleBasedSplitter returned nil")
	}
	if splitter.GetStrategy() != StrategyRuleBased {
		t.Errorf("expected StrategyRuleBased, got %s", splitter.GetStrategy())
	}
}

func TestNewRuleBasedSplitter_NilConfig(t *testing.T) {
	splitter, err := NewRuleBasedSplitter(nil)
	if err != nil {
		t.Fatalf("NewRuleBasedSplitter(nil) failed: %v", err)
	}
	if splitter == nil {
		t.Fatal("NewRuleBasedSplitter(nil) returned nil")
	}
}

func TestRuleBasedSplitter_Split_SimpleQuestion(t *testing.T) {
	cfg := DefaultSplitConfig()
	splitter, _ := NewRuleBasedSplitter(cfg)
	ctx := context.Background()

	subQuestions, err := splitter.Split(ctx, "什么是机器学习？")
	if err != nil {
		t.Fatalf("Split failed: %v", err)
	}
	if len(subQuestions) != 1 {
		t.Errorf("expected 1 sub-question for simple question, got %d", len(subQuestions))
	}
	if subQuestions[0].Content != "什么是机器学习？" {
		t.Errorf("expected original question, got %s", subQuestions[0].Content)
	}
}

func TestRuleBasedSplitter_Split_CompoundQuestion(t *testing.T) {
	cfg := DefaultSplitConfig()
	splitter, _ := NewRuleBasedSplitter(cfg)
	ctx := context.Background()

	question := "什么是机器学习？它有哪些应用？深度学习又是什么？"
	subQuestions, err := splitter.Split(ctx, question)
	if err != nil {
		t.Fatalf("Split failed: %v", err)
	}
	if len(subQuestions) < 2 {
		t.Errorf("expected multiple sub-questions for compound question, got %d", len(subQuestions))
	}
}

func TestRuleBasedSplitter_ShouldSplit(t *testing.T) {
	cfg := DefaultSplitConfig()
	splitter, _ := NewRuleBasedSplitter(cfg)
	ctx := context.Background()

	// Simple question should not split
	shouldSplit, analysis, err := splitter.ShouldSplit(ctx, "你好")
	if err != nil {
		t.Fatalf("ShouldSplit failed: %v", err)
	}
	if shouldSplit {
		t.Error("expected simple question to not require splitting")
	}
	if analysis == nil {
		t.Error("expected analysis to not be nil")
	}

	// Compound question should split
	shouldSplit, _, err = splitter.ShouldSplit(ctx, "什么是机器学习？它有哪些应用？")
	if err != nil {
		t.Fatalf("ShouldSplit failed: %v", err)
	}
	if !shouldSplit {
		t.Error("expected compound question to require splitting")
	}
}

func TestRuleBasedSplitter_Disabled(t *testing.T) {
	cfg := DefaultSplitConfig()
	cfg.Enabled = false
	splitter, _ := NewRuleBasedSplitter(cfg)
	ctx := context.Background()

	shouldSplit, _, err := splitter.ShouldSplit(ctx, "什么是机器学习？它有哪些应用？")
	if err != nil {
		t.Fatalf("ShouldSplit failed: %v", err)
	}
	if shouldSplit {
		t.Error("expected disabled splitter to not require splitting")
	}
}

func TestNewSynthesizer(t *testing.T) {
	cfg := DefaultSynthesisConfig()
	synth, err := NewSynthesizer(cfg)
	if err != nil {
		t.Fatalf("NewSynthesizer failed: %v", err)
	}
	if synth == nil {
		t.Fatal("NewSynthesizer returned nil")
	}
	if synth.GetStrategy() != SynthesisStrategyTemplate {
		t.Errorf("expected SynthesisStrategyTemplate, got %s", synth.GetStrategy())
	}
}

func TestSynthesizer_Synthesize(t *testing.T) {
	cfg := DefaultSynthesisConfig()
	synth, _ := NewSynthesizer(cfg)
	ctx := context.Background()

	// Empty sub-answers should error
	_, err := synth.Synthesize(ctx, "test", nil)
	if err == nil {
		t.Error("expected error for nil sub-answers")
	}

	// Single sub-answer should return directly
	result, err := synth.Synthesize(ctx, "test", []*SubAnswer{
		{QuestionID: "q1", Question: "Q1", Answer: "A1"},
	})
	if err != nil {
		t.Fatalf("Synthesize failed: %v", err)
	}
	if result != "A1" {
		t.Errorf("expected direct return for single answer, got %s", result)
	}

	// Multiple sub-answers should synthesize
	result, err = synth.Synthesize(ctx, "test", []*SubAnswer{
		{QuestionID: "q1", Question: "Q1", Answer: "A1"},
		{QuestionID: "q2", Question: "Q2", Answer: "A2"},
	})
	if err != nil {
		t.Fatalf("Synthesize failed: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty synthesized result")
	}
}

func TestSynthesizer_ValidateSubAnswers(t *testing.T) {
	cfg := DefaultSynthesisConfig()
	synth, _ := NewSynthesizer(cfg)

	// Empty should error
	err := synth.ValidateSubAnswers(nil)
	if err == nil {
		t.Error("expected error for nil sub-answers")
	}

	// Empty answer should error
	err = synth.ValidateSubAnswers([]*SubAnswer{
		{QuestionID: "q1", Question: "Q1", Answer: ""},
	})
	if err == nil {
		t.Error("expected error for empty answer")
	}

	// Valid should pass
	err = synth.ValidateSubAnswers([]*SubAnswer{
		{QuestionID: "q1", Question: "Q1", Answer: "A1"},
	})
	if err != nil {
		t.Errorf("expected no error for valid sub-answers: %v", err)
	}
}

func TestNewComplexityAnalyzer(t *testing.T) {
	cfg := DefaultSplitConfig()
	analyzer := NewComplexityAnalyzer(cfg)
	if analyzer.config == nil {
		t.Error("expected config to be set")
	}
}

func TestComplexityAnalyzer_Analyze(t *testing.T) {
	cfg := DefaultSplitConfig()
	analyzer := NewComplexityAnalyzer(cfg)
	ctx := context.Background()

	analysis, err := analyzer.Analyze(ctx, "什么是机器学习？")
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}
	if analysis == nil {
		t.Fatal("Analyze returned nil")
	}
	if analysis.QuestionType != TypeSimple {
		t.Errorf("expected TypeSimple for simple question, got %s", analysis.QuestionType)
	}

	// Compound question
	analysis, err = analyzer.Analyze(ctx, "什么是机器学习？它有哪些应用？")
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}
	if analysis.QuestionType != TypeCompound {
		t.Errorf("expected TypeCompound for compound question, got %s", analysis.QuestionType)
	}
}

func TestComplexityAnalyzer_ShouldSplit(t *testing.T) {
	cfg := DefaultSplitConfig()
	analyzer := NewComplexityAnalyzer(cfg)

	// Simple question
	analysis := &ComplexAnalysis{
		QuestionType:    TypeSimple,
		ComplexityScore: 0.1,
		LengthScore:     0.1,
	}
	if analyzer.ShouldSplit(analysis, cfg) {
		t.Error("expected simple question to not require splitting")
	}

	// Compound question
	analysis.QuestionType = TypeCompound
	if !analyzer.ShouldSplit(analysis, cfg) {
		t.Error("expected compound question to require splitting")
	}

	// Disabled config
	cfg.Enabled = false
	if analyzer.ShouldSplit(analysis, cfg) {
		t.Error("expected disabled config to not require splitting")
	}
}

func TestQAToCacheEntries(t *testing.T) {
	result := &QASplitResult{
		Split: true,
		QAPairs: []QAPair{
			{Question: "Q1", Answer: "A1"},
			{Question: "Q2", Answer: "A2"},
		},
	}
	entries := QAToCacheEntries(result, "gpt-4")
	if len(entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(entries))
	}
	if entries[0]["model"] != "gpt-4" {
		t.Errorf("expected model gpt-4, got %v", entries[0]["model"])
	}
}

func TestFlattenQA(t *testing.T) {
	result := &QASplitResult{
		QAPairs: []QAPair{
			{Question: "Q1", Answer: "A1"},
			{Question: "Q2", Answer: "A2"},
		},
	}
	questions, answers := FlattenQA(result)
	if len(questions) != 2 || len(answers) != 2 {
		t.Errorf("expected 2 questions and 2 answers, got %d and %d", len(questions), len(answers))
	}
	if questions[0] != "Q1" || answers[0] != "A1" {
		t.Error("expected correct question/answer pairs")
	}
}

func TestFormatQAPair(t *testing.T) {
	pair := QAPair{Question: "Q1", Answer: "A1"}
	formatted := FormatQAPair(pair)
	expected := "Q: Q1\nA: A1"
	if formatted != expected {
		t.Errorf("expected %q, got %q", expected, formatted)
	}
}

func TestConfigVariants(t *testing.T) {
	// HighPerformanceConfig
	hp := HighPerformanceConfig()
	if hp.Split.ComplexityThreshold != 0.7 {
		t.Errorf("expected HP threshold 0.7, got %f", hp.Split.ComplexityThreshold)
	}
	if hp.Synthesis.Strategy != SynthesisStrategyConcat {
		t.Errorf("expected HP synthesis concat, got %s", hp.Synthesis.Strategy)
	}

	// HighQualityConfig
	hq := HighQualityConfig()
	if hq.Split.ComplexityThreshold != 0.4 {
		t.Errorf("expected HQ threshold 0.4, got %f", hq.Split.ComplexityThreshold)
	}
	if hq.Synthesis.Strategy != SynthesisStrategyTemplate {
		t.Errorf("expected HQ synthesis template, got %s", hq.Synthesis.Strategy)
	}

	// MinimalConfig
	min := MinimalConfig()
	if min.Split.Enabled {
		t.Error("expected minimal config to have split disabled")
	}
}

func TestGenerateQuestionID(t *testing.T) {
	id0 := generateQuestionID(0)
	if id0 != "q-0" {
		t.Errorf("expected q-0, got %s", id0)
	}
	id5 := generateQuestionID(5)
	if id5 != "q-5" {
		t.Errorf("expected q-5, got %s", id5)
	}
}
