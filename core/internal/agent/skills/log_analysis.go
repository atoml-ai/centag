package skills

// NewLogAnalysisSkill 创建日志分析Skill
func NewLogAnalysisSkill() *Skill {
	return NewSkill(
		"log-analysis",
		"日志分析",
		"1.0.0",
		"运维诊断",
		[]string{"read_log", "analyze", "centag_info"},
		[]string{
			"调用 centag_info 获取日志文件清单与路径",
			"使用 read_log 读取目标日志（指定 path 与 lines；filter 过滤关键词如 ERROR）",
			"使用 analyze(type=log) 统计错误分布与时间线，输出分析报告：高频错误、趋势、风险等级与处置建议",
		},
	)
}