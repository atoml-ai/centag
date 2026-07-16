package processor

import (
	"context"
	"time"
)

// QuestionProcessor 问题处理器接口
// 定义问题拆分和合成的核心接口，便于后续扩展不同的策略和算法
type QuestionProcessor interface {
	// SplitQuestion 拆分复杂问题为多个子问题
	SplitQuestion(ctx context.Context, question string) (*SplitResult, error)

	// SynthesizeAnswer 合成多个子答案为一个完整答案
	SynthesizeAnswer(ctx context.Context, originalQuestion string, subAnswers []*SubAnswer) (string, error)

	// GetStrategy 获取当前使用的策略
	GetStrategy() SplitStrategy

	// SetStrategy 设置拆分策略
	SetStrategy(strategy SplitStrategy) error

	// GetSplitter 获取底层拆分器（供外部直接调用）
	GetSplitter() QuestionSplitter
}

// QuestionSplitter 问题拆分器接口
// 将复杂问题拆分为多个简单问题，支持多种拆分策略
type QuestionSplitter interface {
	// Split 执行问题拆分
	Split(ctx context.Context, question string) ([]*SubQuestion, error)

	// ShouldSplit 判断是否需要拆分
	ShouldSplit(ctx context.Context, question string) (bool, *ComplexAnalysis, error)

	// GetStrategy 获取拆分策略
	GetStrategy() SplitStrategy
}

// AnswerSynthesizer 答案合成器接口
// 将多个子问题的答案合成为一个完整答案
type AnswerSynthesizer interface {
	// Synthesize 合成答案
	Synthesize(ctx context.Context, originalQuestion string, subAnswers []*SubAnswer) (string, error)

	// GetStrategy 获取合成策略
	GetStrategy() SynthesisStrategy
}

// ComplexityAnalyzer 复杂度分析器接口
// 分析问题的复杂度，决定是否需要拆分
type ComplexityAnalyzer interface {
	// Analyze 分析问题复杂度
	Analyze(ctx context.Context, question string) (*ComplexAnalysis, error)

	// ShouldSplit 根据分析结果判断是否需要拆分
	ShouldSplit(analysis *ComplexAnalysis, config *SplitConfig) bool
}

// LLMService LLM 服务接口
// 用于问题拆分和答案合成的模型调用
type LLMService interface {
	// Generate 生成文本
	Generate(ctx context.Context, prompt string) (string, error)

	// GenerateJSON 生成结构化 JSON 数据
	GenerateJSON(ctx context.Context, prompt string, result interface{}) error

	// GetModelName 获取模型名称
	GetModelName() string

	// GetProvider 获取服务提供商
	GetProvider() string
}

// SplitStrategy 拆分策略类型
type SplitStrategy string

const (
	// StrategyRuleBased 基于规则的拆分策略
	StrategyRuleBased SplitStrategy = "rule_based"
	// StrategyRule 基于规则的拆分策略（别名）
	StrategyRule SplitStrategy = "rule"

	// StrategySemantic 基于语义的拆分策略（需要小模型）
	StrategySemantic SplitStrategy = "semantic"

	// StrategyHybrid 混合策略（规则+语义）
	StrategyHybrid SplitStrategy = "hybrid"
)

// SynthesisStrategy 合成策略类型
type SynthesisStrategy string

const (
	// SynthesisStrategyConcat 简单拼接策略
	SynthesisStrategyConcat SynthesisStrategy = "concat"

	// SynthesisStrategyTemplate 模板合成策略
	SynthesisStrategyTemplate SynthesisStrategy = "template"

	// SynthesisStrategyLLM LLM合成策略（需要大模型重新生成）
	SynthesisStrategyLLM SynthesisStrategy = "llm"

	// SynthesisStrategyHybrid 混合合成策略
	SynthesisStrategyHybrid SynthesisStrategy = "hybrid"
)

// QuestionType 问题类型
type QuestionType string

const (
	// TypeSimple 单一问题
	TypeSimple QuestionType = "simple"

	// TypeCompound 复合问题（包含多个独立问题）
	TypeCompound QuestionType = "compound"

	// TypeChain 链式问题（后续问题依赖前序答案）
	TypeChain QuestionType = "chain"

	// TypeComplex 复杂问题（需要多步骤推理）
	TypeComplex QuestionType = "complex"
)

// SplitResult 拆分结果
type SplitResult struct {
	OriginalQuestion string           `json:"original_question"` // 原始问题
	ShouldSplit      bool             `json:"should_split"`      // 是否需要拆分
	SubQuestions     []*SubQuestion   `json:"sub_questions"`     // 子问题列表
	Complexity       *ComplexAnalysis `json:"complexity"`        // 复杂度分析
	Strategy         SplitStrategy    `json:"strategy"`          // 使用的拆分策略
	Timestamp        time.Time        `json:"timestamp"`         // 拆分时间
}

// SubQuestion 子问题
type SubQuestion struct {
	ID           string   `json:"id"`            // 子问题ID
	Content      string   `json:"content"`       // 子问题内容
	Order        int      `json:"order"`         // 子问题顺序
	ParentID     string   `json:"parent_id"`     // 父问题ID
	Dependencies []string `json:"dependencies"`  // 依赖的子问题ID（用于链式问题）
}

// SubAnswer 子答案
type SubAnswer struct {
	QuestionID string  `json:"question_id"` // 对应的子问题ID
	Question   string  `json:"question"`    // 子问题内容
	Answer     string  `json:"answer"`      // 子问题答案
	Confidence float32 `json:"confidence"`  // 置信度（可选）
	FromCache  bool    `json:"from_cache"`  // 是否来自缓存
}

// ComplexAnalysis 复杂度分析结果
type ComplexAnalysis struct {
	QuestionType        QuestionType `json:"question_type"`         // 问题类型
	ComplexityScore     float32      `json:"complexity_score"`      // 复杂度分数 (0-1)
	LengthScore         float32      `json:"length_score"`          // 长度分数
	StructureScore      float32      `json:"structure_score"`       // 结构分数
	SplitMarkers        []string     `json:"split_markers"`         // 发现的拆分标记
	SuggestedSplitCount int          `json:"suggested_split_count"` // 建议拆分数量
}

// SplitConfig 拆分配置
type SplitConfig struct {
	// 通用配置
	Enabled  bool          `json:"enabled"`  // 是否启用问题拆分
	Strategy SplitStrategy `json:"strategy"` // 拆分策略

	// 复杂度阈值
	ComplexityThreshold float32 `json:"complexity_threshold"` // 复杂度阈值 (0-1)
	MinSplitLength      int     `json:"min_split_length"`     // 最小拆分长度
	MaxSplitCount       int     `json:"max_split_count"`      // 最大拆分数量

	// 规则拆分配置
	SplitMarkers    []string `json:"split_markers"`      // 拆分标记（如 "?", "。", "还有" 等）
	EnableAutoSplit bool     `json:"enable_auto_split"`  // 是否自动拆分
	IgnoreMarkers   []string `json:"ignore_markers"`     // 忽略的标记

	// 语义拆分配置（预留）
	EnableSemantic bool   `json:"enable_semantic"` // 是否启用语义拆分
	SemanticModel  string `json:"semantic_model"`  // 语义模型配置

	// 缓存配置
	CacheSubQuestions bool `json:"cache_sub_questions"` // 是否缓存子问题
	PartialMatch      bool `json:"partial_match"`       // 是否允许部分匹配
}

// SynthesisConfig 合成配置
type SynthesisConfig struct {
	Strategy       SynthesisStrategy `json:"strategy"`        // 合成策略
	Template       string            `json:"template"`        // 合成模板
	EnableCitation bool              `json:"enable_citation"` // 是否添加引用
	PreserveOrder  bool              `json:"preserve_order"`  // 是否保持顺序
	MaxRetry       int               `json:"max_retry"`       // 最大重试次数
}

// ProcessorConfig 处理器配置（包含拆分和合成配置）
type ProcessorConfig struct {
	Split     SplitConfig     `json:"split"`
	Synthesis SynthesisConfig `json:"synthesis"`
}

// QAPair 问答对
type QAPair struct {
	Question string `json:"question"`
	Answer   string `json:"answer"`
}

// QASplitResult 拆分结果
type QASplitResult struct {
	Split   bool     `json:"split"`
	QAPairs []QAPair `json:"qa_pairs"`
}
