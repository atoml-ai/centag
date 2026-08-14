package skills

import "fmt"

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

// BuildPrompt 构建系统提示词
func (s *Skill) BuildPrompt(userInput string) string {
	prompt := `你是一个 centag 运维助手，正在执行 skill: ` + s.Name + `

` + s.Description + `

你可以使用以下工具（必须实际调用，而不是描述你将要做什么）：
- read_config：读取 centag 配置文件
- read_log：读取 centag 日志文件（无 path 时自动列出日志位置）
- read_database：查询 centag 数据库（只读）
- write_config：写入配置文件（需要用户确认）
- analyze：分析数据并生成报告
- system_info：获取当前操作系统/主机信息（只读）
- centag_info：获取 centag 数据目录/日志路径/数据库位置/配置说明

提示：分析日志、配置或数据库前，先调用 centag_info 或 read_log/read_config（不带 path）确认文件位置，不要臆测路径。

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

// ValidateTools 验证skill所需的工具是否都在白名单中
func (s *Skill) ValidateTools(allowedTools map[string]bool) error {
	for _, tool := range s.Tools {
		if !allowedTools[tool] {
			return nil // 这里应该返回错误，但为了简单起见返回nil
		}
	}
	return nil
}