package processor

// DefaultSplitConfig 默认拆分配置
func DefaultSplitConfig() *SplitConfig {
	return &SplitConfig{
		Enabled:             true,
		Strategy:            StrategyRuleBased,
		ComplexityThreshold: 0.5,
		MinSplitLength:      10,
		MaxSplitCount:       10,
		SplitMarkers: []string{
			"?", "？", "。", "；", ";",
			"，", ",", "另外", "此外", "还有", "以及",
		},
		EnableAutoSplit:    true,
		IgnoreMarkers:      []string{},
		EnableSemantic:     false,
		SemanticModel:      "",
		CacheSubQuestions:  true,
		PartialMatch:       false,
	}
}

// DefaultSynthesisConfig 默认合成配置
func DefaultSynthesisConfig() *SynthesisConfig {
	return &SynthesisConfig{
		Strategy:       SynthesisStrategyTemplate,
		Template:       "",
		EnableCitation: true,
		PreserveOrder:  true,
		MaxRetry:       3,
	}
}

// DefaultProcessorConfig 默认处理器配置
func DefaultProcessorConfig() *ProcessorConfig {
	return &ProcessorConfig{
		Split:     *DefaultSplitConfig(),
		Synthesis: *DefaultSynthesisConfig(),
	}
}

// HighPerformanceConfig 高性能配置（减少拆分，简单合成）
func HighPerformanceConfig() *ProcessorConfig {
	splitConfig := &SplitConfig{
		Enabled:             true,
		Strategy:            StrategyRuleBased,
		ComplexityThreshold: 0.7, // 提高阈值，减少拆分
		MinSplitLength:      20,
		MaxSplitCount:       5,
		SplitMarkers:        []string{"？", "?", "。"},
		EnableAutoSplit:     true,
		CacheSubQuestions:   true,
		PartialMatch:        true,
	}

	synthesisConfig := &SynthesisConfig{
		Strategy:       SynthesisStrategyConcat,
		Template:       "",
		EnableCitation: false,
		PreserveOrder:  true,
		MaxRetry:       1,
	}

	return &ProcessorConfig{
		Split:     *splitConfig,
		Synthesis: *synthesisConfig,
	}
}

// HighQualityConfig 高质量配置（智能拆分，模板合成）
func HighQualityConfig() *ProcessorConfig {
	splitConfig := &SplitConfig{
		Enabled:             true,
		Strategy:            StrategyRuleBased,
		ComplexityThreshold: 0.4, // 降低阈值，增加拆分
		MinSplitLength:      10,
		MaxSplitCount:       15,
		SplitMarkers: []string{
			"?", "？", "。", "；", ";",
			"，", ",", "另外", "此外", "还有", "以及",
			"那么", "接下来", "然后",
		},
		EnableAutoSplit:   true,
		CacheSubQuestions: true,
		PartialMatch:      false,
	}

	synthesisConfig := &SynthesisConfig{
		Strategy:       SynthesisStrategyTemplate,
		Template:       "",
		EnableCitation: true,
		PreserveOrder:  true,
		MaxRetry:       3,
	}

	return &ProcessorConfig{
		Split:     *splitConfig,
		Synthesis: *synthesisConfig,
	}
}

// MinimalConfig 最小化配置（只做基础拆分）
func MinimalConfig() *ProcessorConfig {
	splitConfig := &SplitConfig{
		Enabled:             false, // 默认不启用
		Strategy:            StrategyRuleBased,
		ComplexityThreshold: 0.8,
		MinSplitLength:      30,
		MaxSplitCount:       3,
		SplitMarkers:        []string{"？", "?"},
		EnableAutoSplit:     false,
		CacheSubQuestions:   false,
		PartialMatch:        false,
	}

	synthesisConfig := &SynthesisConfig{
		Strategy:       SynthesisStrategyConcat,
		Template:       "",
		EnableCitation: false,
		PreserveOrder:  true,
		MaxRetry:       1,
	}

	return &ProcessorConfig{
		Split:     *splitConfig,
		Synthesis: *synthesisConfig,
	}
}
