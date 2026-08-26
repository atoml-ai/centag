package skills

// NewStatusCheckSkill 创建状态检查Skill
func NewStatusCheckSkill() *Skill {
	return NewSkill(
		"status-check",
		"查询centag当前运行状态",
		"1.0.0",
		"运维诊断",
		[]string{"read_config", "centag_info", "read_database", "analyze"},
		[]string{
			"调用 centag_info 获取数据目录结构、日志路径与可用数据表清单",
			"使用 read_config 读取主配置 centag.conf 与 proxy-config.yaml，关注 server/proxy/logging 段",
			"使用 read_database 查询运行状态表：agent_sessions(会话量)、user_request_logs(请求量)、token_usage(用量)",
			"使用 analyze(type=status) 综合分析，输出健康报告：服务状态、流量趋势、异常指标与处置建议",
		},
	)
}