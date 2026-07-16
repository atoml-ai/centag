package expansion

import (
	"context"
	"regexp"
	"sort"
	"strings"

	"centag/core/pkg/plugin"
)

// RuleBasedExpander 基于规则的查询展开器
type RuleBasedExpander struct {
	config *RuleConfig

	// 指代词到权重的映射
	pronounWeights map[string]float64

	// 实体提取器列表
	entityExtractors []EntityExtractor
}

// EntityExtractor 实体提取器接口
type EntityExtractor interface {
	// Extract 从文本中提取实体
	Extract(text string) []Entity
	// Priority 优先级（数字越小优先级越高）
	Priority() int
	// Name 提取器名称
	Name() string
}

// Entity 表示提取的实体
type Entity struct {
	Text     string  // 实体文本
	Type     string  // 实体类型：topic | concept | keyword | person | organization
	Position int     // 在文本中的位置
	Score    float64 // 置信度分数
}

// NewRuleBasedExpander 创建规则展开器
func NewRuleBasedExpander(config *RuleConfig) (*RuleBasedExpander, error) {
	if config == nil {
		config = &RuleConfig{}
	}

	// 设置默认值
	if len(config.Pronouns) == 0 {
		config.Pronouns = defaultPronouns()
	}
	if config.MaxHistoryRounds == 0 {
		config.MaxHistoryRounds = 3
	}
	if config.MinEntityScore == 0 {
		config.MinEntityScore = 0.5
	}

	expander := &RuleBasedExpander{
		config:         config,
		pronounWeights: buildPronounWeights(config.Pronouns),
		entityExtractors: []EntityExtractor{
			&QuotedTextExtractor{},    // 引号中的文本（最高优先级）
			&TitleCaseExtractor{},     // 大写开头的专有名词
			&NounPhraseExtractor{},    // 名词短语
			&KeywordExtractor{},       // 关键词
		},
	}

	// 按优先级排序提取器
	sort.Slice(expander.entityExtractors, func(i, j int) bool {
		return expander.entityExtractors[i].Priority() < expander.entityExtractors[j].Priority()
	})

	return expander, nil
}

// Expand 展开查询
func (rbe *RuleBasedExpander) Expand(
	ctx context.Context,
	current string,
	history []plugin.Message,
) (string, bool, error) {
	if !rbe.config.Enabled {
		return current, false, nil
	}

	// 1. 检查是否需要展开（检测指代词）
	pronouns := rbe.detectPronouns(current)
	if len(pronouns) == 0 {
		return current, false, nil
	}

	// 2. 从历史中提取候选实体
	entities := rbe.extractEntitiesFromHistory(history)
	if len(entities) == 0 {
		return current, false, nil
	}

	// 3. 对实体进行排序和过滤
	entities = rbe.rankEntities(entities)

	// 4. 执行替换
	expanded := current
	usedEntities := make(map[string]bool)

	for _, pronoun := range pronouns {
		if usedEntities[pronoun] {
			continue
		}

		// 找到最佳匹配的实体
		entity := rbe.findBestMatch(pronoun, entities, usedEntities)
		if entity != nil {
			expanded = strings.Replace(expanded, pronoun, entity.Text, 1)
			usedEntities[entity.Text] = true
		}
	}

	isExpanded := expanded != current

	return expanded, isExpanded, nil
}

// Name 返回展开器名称
func (rbe *RuleBasedExpander) Name() string {
	return "rule_based"
}

// detectPronouns 检测查询中的指代词
func (rbe *RuleBasedExpander) detectPronouns(text string) []string {
	found := make([]string, 0)
	lowerText := strings.ToLower(text)

	for pronoun := range rbe.pronounWeights {
		if strings.Contains(lowerText, strings.ToLower(pronoun)) {
			found = append(found, pronoun)
		}
	}

	return found
}

// extractEntitiesFromHistory 从历史消息中提取实体
func (rbe *RuleBasedExpander) extractEntitiesFromHistory(history []plugin.Message) []Entity {
	allEntities := make([]Entity, 0)

	// 只取最近 N 轮对话
	startIdx := 0
	if len(history) > rbe.config.MaxHistoryRounds*2 {
		startIdx = len(history) - rbe.config.MaxHistoryRounds*2
	}

	// 倒序遍历，越近的消息权重越高
	for i := len(history) - 1; i >= startIdx; i-- {
		msg := history[i]

		// 只从助手的回答中提取实体（通常包含更多关键信息）
		if msg.Role != "assistant" {
			continue
		}

		// 计算距离衰减因子
		distance := len(history) - 1 - i
		distanceDecay := 1.0 / (1.0 + float64(distance)*0.3)

		// 使用所有提取器提取实体
		for _, extractor := range rbe.entityExtractors {
			entities := extractor.Extract(msg.Content)

			// 应用距离衰减
			for j := range entities {
				entities[j].Score *= distanceDecay
			}

			allEntities = append(allEntities, entities...)
		}
	}

	return allEntities
}

// rankEntities 对实体进行排序和过滤
func (rbe *RuleBasedExpander) rankEntities(entities []Entity) []Entity {
	// 按分数降序排序
	sort.Slice(entities, func(i, j int) bool {
		return entities[i].Score > entities[j].Score
	})

	// 过滤低分实体并去重
	seen := make(map[string]bool)
	filtered := make([]Entity, 0)

	for _, e := range entities {
		if e.Score < rbe.config.MinEntityScore {
			continue
		}

		key := strings.ToLower(e.Text)
		if seen[key] {
			continue
		}

		seen[key] = true
		filtered = append(filtered, e)
	}

	return filtered
}

// findBestMatch 找到最佳匹配的实体
func (rbe *RuleBasedExpander) findBestMatch(
	pronoun string,
	entities []Entity,
	used map[string]bool,
) *Entity {
	var best *Entity
	bestScore := 0.0

	pronounWeight := rbe.pronounWeights[strings.ToLower(pronoun)]

	for i := range entities {
		e := &entities[i]

		if used[e.Text] {
			continue
		}

		// 计算匹配分数
		score := e.Score

		// 根据指代词类型调整权重
		switch pronounWeight {
		case 3.0: // 强指代词（它、这、那等）
			if e.Type == "topic" || e.Type == "concept" {
				score *= 1.5
			}
		case 2.0: // 人称指代（他、她等）
			if e.Type == "person" {
				score *= 2.0
			}
		case 1.0: // 地点指代（这里、那里等）
			if e.Type == "location" {
				score *= 2.0
			}
		}

		// 优先选择较长的实体（通常更具体）
		lengthBonus := float64(len(e.Text)) / 100.0
		score += lengthBonus

		if score > bestScore {
			bestScore = score
			best = e
		}
	}

	return best
}

// buildPronounWeights 构建指代词权重映射
func buildPronounWeights(pronouns []string) map[string]float64 {
	weights := make(map[string]float64)

	// 定义不同类别的指代词及其权重
	strongPronouns := []string{"它", "这", "那", "这个", "那个", "上述", "该", "此", "前者", "后者"}
	personPronouns := []string{"他", "她", "他们", "她们", "它们"}
	locationPronouns := []string{"这里", "那里", "这边", "那边"}

	for _, p := range pronouns {
		lowerP := strings.ToLower(p)
		weights[lowerP] = 1.0 // 默认权重

		// 根据类别设置权重
		for _, sp := range strongPronouns {
			if lowerP == strings.ToLower(sp) {
				weights[lowerP] = 3.0
				break
			}
		}

		for _, pp := range personPronouns {
			if lowerP == strings.ToLower(pp) {
				weights[lowerP] = 2.0
				break
			}
		}

		for _, lp := range locationPronouns {
			if lowerP == strings.ToLower(lp) {
				weights[lowerP] = 1.5
				break
			}
		}
	}

	return weights
}

// defaultPronouns 返回默认指代词列表
func defaultPronouns() []string {
	return []string{
		// 中文 - 强指代
		"它", "这", "那", "这个", "那个",
		"前者", "后者", "上述", "该", "此",
		// 中文 - 人称
		"他", "她", "他们", "她们", "它们",
		// 中文 - 地点
		"这里", "那里", "这边", "那边",
		// 英文 - 强指代
		"it", "this", "that", "these", "those",
		// 英文 - 人称
		"he", "she", "they", "them", "him", "her",
		// 英文 - 地点
		"here", "there",
	}
}

// ==================== 实体提取器实现 ====================

// QuotedTextExtractor 提取引号中的文本
type QuotedTextExtractor struct{}

func (e *QuotedTextExtractor) Name() string { return "quoted_text" }
func (e *QuotedTextExtractor) Priority() int { return 1 }

func (e *QuotedTextExtractor) Extract(text string) []Entity {
	entities := make([]Entity, 0)

	// 匹配各种引号
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`"([^"]{2,30})"`),
		regexp.MustCompile(`'([^']{2,30})'`),
		regexp.MustCompile(`「([^」]{2,30})」`),
		regexp.MustCompile(`『([^』]{2,30})』`),
		regexp.MustCompile(`《([^》]{2,30})》`),
	}

	for _, pattern := range patterns {
		matches := pattern.FindAllStringSubmatchIndex(text, -1)
		for _, match := range matches {
			if len(match) >= 4 {
				content := text[match[2]:match[3]]
				entities = append(entities, Entity{
					Text:     content,
					Type:     "concept",
					Position: match[0],
					Score:    1.2,
				})
			}
		}
	}

	return entities
}

// TitleCaseExtractor 提取大写开头的词组
type TitleCaseExtractor struct{}

func (e *TitleCaseExtractor) Name() string { return "title_case" }
func (e *TitleCaseExtractor) Priority() int { return 2 }

func (e *TitleCaseExtractor) Extract(text string) []Entity {
	entities := make([]Entity, 0)

	// 匹配连续的大写开头词（专有名词）
	pattern := regexp.MustCompile(`([A-Z][a-zA-Z]+(?:\s+[A-Z][a-zA-Z]+)+)`)
	matches := pattern.FindAllStringSubmatchIndex(text, -1)

	for _, match := range matches {
		if len(match) >= 2 {
			content := text[match[0]:match[1]]
			entities = append(entities, Entity{
				Text:     content,
				Type:     "topic",
				Position: match[0],
				Score:    1.0,
			})
		}
	}

	return entities
}

// NounPhraseExtractor 提取名词短语
type NounPhraseExtractor struct{}

func (e *NounPhraseExtractor) Name() string { return "noun_phrase" }
func (e *NounPhraseExtractor) Priority() int { return 3 }

func (e *NounPhraseExtractor) Extract(text string) []Entity {
	entities := make([]Entity, 0)

	// 匹配常见的名词短语模式（使用中文字符类）
	patterns := []*regexp.Regexp{
		// XX技术/语言/框架/工具 - 匹配2-8个中文字符后跟特定后缀
		regexp.MustCompile(`([^\x00-\x7F]{2,8}(?:技术|语言|框架|工具|方法|算法|模型|系统))`),
		// XX开发/编程/设计
		regexp.MustCompile(`([^\x00-\x7F]{2,8}(?:开发|编程|设计|测试|部署))`),
		// 英文技术术语
		regexp.MustCompile(`\b([A-Z][a-z]+(?:\.[A-Z][a-z]+)*)\b`),
	}

	for _, pattern := range patterns {
		matches := pattern.FindAllStringSubmatchIndex(text, -1)
		for _, match := range matches {
			if len(match) >= 2 {
				content := text[match[0]:match[1]]
				entities = append(entities, Entity{
					Text:     content,
					Type:     "topic",
					Position: match[0],
					Score:    0.9,
				})
			}
		}
	}

	return entities
}

// KeywordExtractor 关键词提取器
type KeywordExtractor struct{}

func (e *KeywordExtractor) Name() string { return "keyword" }
func (e *KeywordExtractor) Priority() int { return 4 }

func (e *KeywordExtractor) Extract(text string) []Entity {
	entities := make([]Entity, 0)

	// 提取定义类问题的主题（使用非ASCII字符匹配中文）
	patterns := []*regexp.Regexp{
		// "什么是XXX" / "XXX是什么"
		regexp.MustCompile(`(?:什么是|啥是)([^\x00-\x7F]{2,10}?)(?:[？?]|$)`),
		regexp.MustCompile(`([^\x00-\x7F]{2,10}?)(?:是什么|是啥)`),
		// "如何/怎么XXX"
		regexp.MustCompile(`(?:如何|怎么|怎样)([^\x00-\x7F]{2,10}?)(?:[？?]|$)`),
		// "介绍/解释XXX" - 使用非贪婪匹配并限制后续字符
		regexp.MustCompile(`(?:介绍|解释|说明|谈谈)([^\x00-\x7F]{2,8}?)(?:[。.！!]|$)`),
	}

	for _, pattern := range patterns {
		matches := pattern.FindAllStringSubmatchIndex(text, -1)
		for _, match := range matches {
			if len(match) >= 4 {
				content := text[match[2]:match[3]]
				entities = append(entities, Entity{
					Text:     content,
					Type:     "keyword",
					Position: match[0],
					Score:    0.85,
				})
			}
		}
	}

	return entities
}
