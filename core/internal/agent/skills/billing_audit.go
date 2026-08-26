package skills

// NewBillingAuditSkill 创建计费审计Skill
func NewBillingAuditSkill() *Skill {
	return NewSkill(
		"billing-audit",
		"计费审计",
		"1.0.0",
		"计费管理",
		[]string{"read_database", "analyze", "centag_info"},
		[]string{
			"调用 centag_info 获取数据目录与可用数据表清单",
			"使用 read_database 查询 token_usage(用量明细)、token_usage_daily(按天聚合)、token_quotas(配额)，核对计费数据",
			"使用 read_database 查询 pricing_rules(计费规则)，比对单价与用量，复核费用准确性",
			"使用 analyze(type=status) 输出计费审计报告：费用核算、异常用量(超额/未定价模型)、差异项与整改建议",
		},
	)
}
