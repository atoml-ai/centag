package skills

// NewStrategyRecommendSkill 创建策略调整建议Skill
func NewStrategyRecommendSkill() *Skill {
	return NewSkill(
		"strategy-recommend",
		"策略调整建议",
		"1.0.0",
		"策略优化",
		[]string{"read_config", "read_database", "analyze"},
		[]string{
			"使用read_config工具读取当前配置",
			"使用read_database工具查询运行时数据",
			"使用analyze工具分析策略效果",
			"输出优化建议报告",
		},
	)
}