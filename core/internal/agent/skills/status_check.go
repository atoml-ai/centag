package skills

// NewStatusCheckSkill 创建状态检查Skill
func NewStatusCheckSkill() *Skill {
	return NewSkill(
		"status-check",
		"查询centag当前运行状态",
		"1.0.0",
		"运维诊断",
		[]string{"read_config", "read_database", "analyze"},
		[]string{
			"使用read_config工具读取配置文件",
			"使用read_database工具查询运行时状态",
			"使用analyze工具分析状态",
			"输出状态报告",
		},
	)
}