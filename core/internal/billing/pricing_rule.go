package billing

import (
	"centag/core/pkg/billing"
)

// 重新导出 pkg/billing 中的类型，保持内部包的兼容性
type PriceType = billing.PriceType
type PricingRule = billing.PricingRule
type PricingRulesFile = billing.PricingRulesFile
type PriceInfo = billing.PriceInfo
type CostBreakdown = billing.CostBreakdown

// 重新导出常量
const (
	PriceTypeCost    = billing.PriceTypeCost
	PriceTypeRevenue = billing.PriceTypeRevenue
)

const (
	PriceSourceRule        = billing.PriceSourceRule
	PriceSourceLegacyTable = billing.PriceSourceLegacyTable
	PriceSourceDefault     = billing.PriceSourceDefault
	PriceSourceConfig      = billing.PriceSourceConfig
)

const DefaultPricingCurrency = billing.DefaultPricingCurrency
