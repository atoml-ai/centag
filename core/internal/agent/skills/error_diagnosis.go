package skills

// NewErrorDiagnosisSkill 创建错误诊断Skill
func NewErrorDiagnosisSkill() *Skill {
	return NewSkill(
		"error-diagnosis",
		"错误诊断",
		"1.0.0",
		"运维诊断",
		[]string{"read_log", "read_database", "analyze"},
		[]string{
			"使用read_log工具读取最近的日志",
			"使用read_database工具查询相关状态",
			"使用analyze工具分析错误原因",
			"输出诊断报告和解决方案",
		},
	)
}