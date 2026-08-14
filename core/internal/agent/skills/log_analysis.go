package skills

// NewLogAnalysisSkill 创建日志分析Skill
func NewLogAnalysisSkill() *Skill {
	return NewSkill(
		"log-analysis",
		"日志分析",
		"1.0.0",
		"运维诊断",
		[]string{"read_log", "analyze"},
		[]string{
			"使用read_log工具读取日志文件",
			"使用analyze工具分析日志",
			"输出分析报告",
		},
	)
}