package skills

// NewConfigAnalysisSkill 创建配置分析Skill
func NewConfigAnalysisSkill() *Skill {
	return NewSkill(
		"config-analysis",
		"分析当前配置",
		"1.0.0",
		"配置管理",
		[]string{"read_config", "analyze"},
		[]string{
			"使用read_config工具读取配置文件",
			"使用analyze工具分析配置",
			"输出分析报告和优化建议",
		},
	)
}