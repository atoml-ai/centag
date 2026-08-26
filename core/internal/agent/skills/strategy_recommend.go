package skills

// NewStrategyRecommendSkill 创建策略调整建议Skill
func NewStrategyRecommendSkill() *Skill {
	return NewSkill(
		"strategy-recommend",
		"策略调整建议",
		"1.0.0",
		"策略优化",
		[]string{"read_config", "centag_info", "read_database", "analyze"},
		[]string{
			"调用 centag_info 获取系统概览与可用数据表清单",
			"使用 read_config 读取当前基线配置：centag.conf 与 proxy-config.yaml",
			"使用 read_database 查询 user_request_logs(请求量)、token_usage(用量)、pricing_rules(计费规则)，分析流量与成本结构",
			"使用 analyze(type=strategy) 评估策略效果，输出优化建议：路由策略、成本控制、容量规划与预期收益",
		},
	)
}