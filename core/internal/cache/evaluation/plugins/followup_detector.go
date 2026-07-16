package plugins

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"centag/core/internal/cache/evaluation/plugin"
)

// FollowUpDetectorPlugin 追问检测插件
type FollowUpDetectorPlugin struct {
	config *FollowUpConfig
}

// FollowUpConfig 追问检测配置
type FollowUpConfig struct {
	Enabled        bool     `json:"enabled" default:"true"`
	Pronouns       []string `json:"pronouns"`
	ShortThreshold int      `json:"short_threshold" default:"15"`
	ScoreThreshold float64  `json:"score_threshold" default:"3"`
}

// NewFollowUpDetectorPlugin 创建追问检测插件
func NewFollowUpDetectorPlugin() plugin.EvaluatorPlugin {
	return &FollowUpDetectorPlugin{
		config: &FollowUpConfig{
			Pronouns: defaultPronouns(),
		},
	}
}

// Name 返回插件名称
func (p *FollowUpDetectorPlugin) Name() string {
	return "follow_up_detector"
}

// Version 返回插件版本
func (p *FollowUpDetectorPlugin) Version() string {
	return "1.0.0"
}

// Type 返回插件类型
func (p *FollowUpDetectorPlugin) Type() plugin.PluginType {
	return plugin.PluginTypeInput
}

// Description 返回插件描述
func (p *FollowUpDetectorPlugin) Description() string {
	return "检测问题是否为追问，基于指代词、长度、模式等特征"
}

// Init 初始化插件
func (p *FollowUpDetectorPlugin) Init() error {
	// 设置默认值
	if len(p.config.Pronouns) == 0 {
		p.config.Pronouns = defaultPronouns()
	}
	if p.config.ScoreThreshold == 0 {
		p.config.ScoreThreshold = 3 // 默认阈值
	}
	if p.config.ShortThreshold == 0 {
		p.config.ShortThreshold = 15 // 默认短问题阈值
	}
	return nil
}

// Close 关闭插件
func (p *FollowUpDetectorPlugin) Close() error {
	return nil
}

// HealthCheck 健康检查
func (p *FollowUpDetectorPlugin) HealthCheck() error {
	return nil
}

// Evaluate 执行评估
func (p *FollowUpDetectorPlugin) Evaluate(
	ctx context.Context,
	input *plugin.EvalInput,
) (*plugin.EvalOutput, error) {
	start := time.Now()

	score := 0.0
	labels := make([]string, 0)
	details := make(map[string]interface{})

	question := input.OriginalQuestion
	if question == "" {
		question = input.Question
	}

	// 1. 历史消息检查 - 第一轮不可能是追问
	if len(input.HistoryMessages) < 2 {
		return &plugin.EvalOutput{
			Score:         100,
			Passed:        true,
			Labels:        []string{"first_round"},
			Details:       map[string]interface{}{"reason": "no_history"},
			ProcessTimeMs: time.Since(start).Milliseconds(),
			Metadata: map[string]interface{}{
				"is_follow_up": false,
			},
		}, nil
	}

	// 2. 指代词检测
	pronouns := p.detectPronouns(question)
	if len(pronouns) > 0 {
		score += 3
		labels = append(labels, "has_pronouns")
		details["pronouns"] = pronouns
		details["pronoun_count"] = len(pronouns)
	}

	// 3. 短问题检测
	charCount := utf8.RuneCountInString(question)
	if charCount < p.config.ShortThreshold {
		score += 1
		labels = append(labels, "short_question")
		details["char_count"] = charCount
	}

	// 4. 追问模式检测
	patterns := []struct {
		pattern string
		score   float64
		label   string
	}{
		{`^(请)?(详细|具体|展开|深入)(说说|解释|介绍|说明|谈谈)`, 2, "detail_request"},
		{`^(那|那么|所以|然后|接着|还有)(呢|吗)?$`, 2, "continuation"},
		{`^(继续|接着|还有|另外|除此之外)(呢)?`, 2, "continuation"},
		{`^[第一二三四五六七八九十]+[个种点步]$`, 1, "enumeration"},
		{`^[第][一二三四五六七八九十].*呢`, 2, "sequence_ask"},
		{`(为什么|怎么|怎样).*?(这样|那样|它|这)`, 2, "referential_why"},
	}

	for _, pt := range patterns {
		if matched, _ := regexp.MatchString(pt.pattern, question); matched {
			score += pt.score
			labels = append(labels, pt.label)
			details["matched_pattern"] = pt.pattern
			break // 只匹配第一个
		}
	}

	// 5. 上下文引用检测
	refPatterns := []string{
		`上述`, `前面提到的`, `刚才说的`, `之前讨论的`,
		`前者`, `后者`, `这个`, `那个`, `这些`, `那些`,
	}
	refCount := 0
	for _, ref := range refPatterns {
		if strings.Contains(question, ref) {
			refCount++
		}
	}
	if refCount > 0 {
		score += float64(refCount) * 1.5
		details["reference_count"] = refCount
	}

	isFollowUp := score >= p.config.ScoreThreshold

	// 计算最终分数（转换为0-100）
	finalScore := 100.0 - score*10
	if finalScore < 0 {
		finalScore = 0
	}

	return &plugin.EvalOutput{
		Score:         finalScore,
		Passed:        !isFollowUp, // 追问不通过（需要特殊处理）
		Labels:        labels,
		Details:       details,
		ProcessTimeMs: time.Since(start).Milliseconds(),
		Metadata: map[string]interface{}{
			"is_follow_up":    isFollowUp,
			"raw_score":       score,
			"char_count":      charCount,
			"history_length":  len(input.HistoryMessages),
		},
	}, nil
}

// detectPronouns 检测查询中的指代词
func (p *FollowUpDetectorPlugin) detectPronouns(text string) []string {
	found := make([]string, 0)
	lowerText := strings.ToLower(text)

	for _, pronoun := range p.config.Pronouns {
		if strings.Contains(lowerText, strings.ToLower(pronoun)) {
			found = append(found, pronoun)
		}
	}
	return found
}

// GetConfigSchema 获取配置schema
func (p *FollowUpDetectorPlugin) GetConfigSchema() *plugin.ConfigSchema {
	return &plugin.ConfigSchema{
		Fields: []plugin.ConfigField{
			{
				Name:        "short_threshold",
				Type:        "number",
				Description: "短问题字符阈值",
				Required:    false,
				Default:     15,
				Min:         ptrFloat64(5),
				Max:         ptrFloat64(50),
			},
			{
				Name:        "score_threshold",
				Type:        "number",
				Description: "判定为追问的分数阈值",
				Required:    false,
				Default:     3,
				Min:         ptrFloat64(1),
				Max:         ptrFloat64(10),
			},
			{
				Name:        "pronouns",
				Type:        "array",
				Description: "自定义指代词列表",
				Required:    false,
				Default:     defaultPronouns(),
			},
		},
	}
}

// ValidateConfig 验证配置
func (p *FollowUpDetectorPlugin) ValidateConfig(config map[string]interface{}) error {
	if threshold, ok := config["score_threshold"].(float64); ok {
		if threshold < 1 || threshold > 10 {
			return fmt.Errorf("score_threshold must be between 1 and 10")
		}
	}
	return nil
}

// SetConfig 设置配置
func (p *FollowUpDetectorPlugin) SetConfig(config map[string]interface{}) error {
	if err := p.ValidateConfig(config); err != nil {
		return err
	}

	if enabled, ok := config["enabled"].(bool); ok {
		p.config.Enabled = enabled
	}
	if threshold, ok := config["score_threshold"].(float64); ok {
		p.config.ScoreThreshold = threshold
	}
	if shortThreshold, ok := config["short_threshold"].(float64); ok {
		p.config.ShortThreshold = int(shortThreshold)
	}
	if pronouns, ok := config["pronouns"].([]string); ok {
		p.config.Pronouns = pronouns
	}

	return nil
}

// GetConfig 获取当前配置
func (p *FollowUpDetectorPlugin) GetConfig() map[string]interface{} {
	return map[string]interface{}{
		"enabled":         p.config.Enabled,
		"score_threshold": p.config.ScoreThreshold,
		"short_threshold": p.config.ShortThreshold,
		"pronouns":        p.config.Pronouns,
	}
}

// defaultPronouns 默认指代词列表
func defaultPronouns() []string {
	return []string{
		// 中文
		"它", "这", "那", "这个", "那个",
		"他", "她", "他们", "她们", "它们",
		"前者", "后者", "上述", "该", "此",
		"这里", "那里", "这边", "那边",
		// 英文
		"it", "this", "that", "these", "those",
		"he", "she", "they", "them", "him", "her",
		"here", "there",
	}
}
