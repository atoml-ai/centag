package skills

// NewConfigAnalysisSkill 创建配置分析Skill
func NewConfigAnalysisSkill() *Skill {
	return NewSkill(
		"config-analysis",
		"分析当前配置",
		"1.0.0",
		"配置管理",
		[]string{"read_config", "analyze", "centag_info"},
		[]string{
			"调用 centag_info 了解配置文件清单与说明",
			"使用 read_config 分别读取 centag.conf（server/database/proxy/logging 段）与 proxy-config.yaml（后端与路由）",
			"使用 analyze(type=config) 校验配置一致性，输出报告：当前生效值、潜在问题、优化建议",
		},
	)
}