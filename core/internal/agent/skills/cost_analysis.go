package skills

// NewCostAnalysisSkill 创建成本分析Skill
func NewCostAnalysisSkill() *Skill {
	return NewSkill(
		"cost-analysis",
		"成本分析",
		"1.0.0",
		"计费管理",
		[]string{"read_database", "analyze", "centag_info"},
		[]string{
			"调用 centag_info 获取数据目录与可用数据表清单",
			"使用 read_database 查询 token_usage/token_usage_daily 按 backend_id/model/user_id 聚合用量，并读取 pricing_rules(单价)",
			"使用 analyze(type=strategy) 计算成本结构：按模型/后端/用户拆分，识别高成本项与异常费用",
			"输出成本分析报告：成本构成、趋势、优化建议(降本/换模型/配额控制)与预期收益",
		},
	)
}
