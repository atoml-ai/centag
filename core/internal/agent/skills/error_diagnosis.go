package skills

// NewErrorDiagnosisSkill 创建错误诊断Skill
func NewErrorDiagnosisSkill() *Skill {
	return NewSkill(
		"error-diagnosis",
		"错误诊断",
		"1.0.0",
		"运维诊断",
		[]string{"read_log", "read_database", "analyze", "centag_info"},
		[]string{
			"调用 centag_info 确认日志文件位置与可用的诊断数据源",
			"使用 read_log 读取 centag.log 最近日志（无 path 时会列出候选；可加 filter=ERROR|WARN 过滤）",
			"若日志为空或读取失败：使用 read_config 检查 logging 段（级别/输出路径），提示用户先修正日志配置再重试",
			"使用 read_database 查询 user_request_logs(请求记录)、token_usage(用量)，定位异常时段的请求与错误分布",
			"使用 analyze(type=error) 归因，输出诊断报告：根因、影响面、修复步骤与预防建议",
		},
	)
}