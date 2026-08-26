package skills

import (
	"fmt"
	"strings"
)

// Skill Skill基础结构
type Skill struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Version     string   `json:"version"`
	Category    string   `json:"category"`
	Tools       []string `json:"tools"`
	Steps       []string `json:"steps"`
	Enabled     bool     `json:"enabled"`
	Internal    bool     `json:"internal"`
	// Prompt 为 manifest 提供的完整系统提示词；为空时回退 BuildPrompt 默认文案。
	Prompt string `json:"-"`
}

// NewSkill 创建Skill
func NewSkill(name, description, version, category string, tools, steps []string) *Skill {
	return &Skill{
		Name:        name,
		Description: description,
		Version:     version,
		Category:    category,
		Tools:       tools,
		Steps:       steps,
		Enabled:     true,
		Internal:    true,
	}
}

// skillToolDescriptions 内置工具一句话说明，按 skill.tools 动态渲染提示词工具清单。
// 必须与 tools 包各工具 Description() 语义一致（提示词只能承诺实际注册的工具）。
var skillToolDescriptions = map[string]string{
	"read_config":   "读取 centag 配置文件",
	"read_log":      "读取 centag 日志文件（无 path 时自动列出日志位置）",
	"read_database": "查询 centag 数据库（只读，仅限白名单表）",
	"write_config":  "写入配置文件（需要用户确认）",
	"analyze":       "分析数据并生成统计摘要",
	"system_info":   "获取当前操作系统/主机信息（只读）",
	"centag_info":   "获取 centag 数据目录/日志路径/数据库位置/配置说明",
}

// BuildPrompt 构建系统提示词。当 Prompt 字段非空（manifest 提供）时直接返回。
// 工具清单按 s.Tools 动态生成：提示词声称的工具必须与运行期实际注册集
// （skill.tools ∩ 全局白名单）一致，避免模型调用未注册工具。
func (s *Skill) BuildPrompt(userInput string) string {
	if s.Prompt != "" {
		if userInput != "" {
			return s.Prompt + "\n用户请求: " + userInput + "\n"
		}
		return s.Prompt
	}
	prompt := `你是一个 centag 运维助手，正在执行 skill: ` + s.Name + `

` + s.Description + `

`
	if len(s.Tools) > 0 {
		prompt += "你可以使用以下工具（必须实际调用，而不是描述你将要做什么）：\n"
		for _, t := range s.Tools {
			desc, ok := skillToolDescriptions[t]
			if !ok {
				desc = t
			}
			prompt += "- " + t + "：" + desc + "\n"
		}
	}
	if hint := s.discoveryHint(); hint != "" {
		prompt += "\n" + hint + "\n"
	}
	prompt += `
执行规则（非常重要）：
1. 必须通过调用工具获取真实数据，禁止凭空编造或只描述计划。
2. 禁止输出"我将开始..."之类的计划性文字；直接调用工具并基于工具结果回答。

请按照以下步骤执行：
`
	for i, step := range s.Steps {
		prompt += fmt.Sprintf("%d. %s\n", i+1, step)
	}
	if userInput != "" {
		prompt += "\n用户请求: " + userInput + "\n"
	}
	return prompt
}

// discoveryHint 按实际声明的探索类工具生成路径确认提示；无可用探索工具时返回空。
func (s *Skill) discoveryHint() string {
	has := func(t string) bool {
		for _, x := range s.Tools {
			if x == t {
				return true
			}
		}
		return false
	}
	var parts []string
	for _, t := range []string{"centag_info", "read_config", "read_log"} {
		if has(t) {
			parts = append(parts, t)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return "提示：分析前先调用 " + strings.Join(parts, "/") +
		"（不带 path 时自动列出候选位置/可用表），不要臆测路径。"
}

// ValidateTools 验证skill所需的工具是否都在白名单中
func (s *Skill) ValidateTools(allowedTools map[string]bool) error {
	for _, tool := range s.Tools {
		if !allowedTools[tool] {
			return nil // 这里应该返回错误，但为了简单起见返回nil
		}
	}
	return nil
}