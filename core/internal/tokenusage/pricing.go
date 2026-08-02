package tokenusage

import (
	"context"

	billinginternal "centag/core/internal/billing"
	billingpkg "centag/core/pkg/billing"
	"centag/core/pkg/scheduler"
)

// priceTable is the deprecated hardcoded fallback (often CNY despite cost_usd column name).
var priceTable = scheduler.NewModelPriceTable()

var pricingSvc billinginternal.PricingService

// SetPricingService injects the billing PricingService used for cost attribution.
// Safe to call with nil to clear (tests / teardown).
func SetPricingService(s billinginternal.PricingService) {
	pricingSvc = s
}

// PricingBreakdown is the detailed estimate used when writing usage rows.
type PricingBreakdown struct {
	TotalCost     float64
	InputCost     float64
	OutputCost    float64
	Currency      string
	PricingRuleID int64
	Source        string
	PriceType     billingpkg.PriceType
}

// EstimateCost computes prompt + completion cost.
// Prefer PricingService when injected; otherwise fall back to the legacy price table.
// Failures in PricingService fall back silently (do not block the proxy path).
func EstimateCost(backendID, model string, promptTokens, completionTokens int) float64 {
	bd := EstimateCostDetailed(backendID, model, promptTokens, completionTokens)
	return bd.TotalCost
}

// EstimateCostDetailed returns a full cost breakdown for RecordUsage.
func EstimateCostDetailed(backendID, model string, promptTokens, completionTokens int) PricingBreakdown {
	if promptTokens <= 0 && completionTokens <= 0 {
		return PricingBreakdown{Currency: billinginternal.DefaultPricingCurrency, Source: "zero"}
	}
	if pricingSvc != nil {
		bd, err := pricingSvc.EstimateCost(context.Background(), backendID, model, promptTokens, completionTokens)
		if err == nil && bd != nil {
			return PricingBreakdown{
				TotalCost:     bd.TotalCost,
				InputCost:     bd.InputCost,
				OutputCost:    bd.OutputCost,
				Currency:      bd.Currency,
				PricingRuleID: bd.PricingRuleID,
				Source:        bd.Source,
				PriceType:     bd.PriceType,
			}
		}
	}
	total := priceTable.EstimateCost(backendID, model, promptTokens, completionTokens)
	price := priceTable.GetPrice(backendID, model)
	in := float64(promptTokens) / 1_000_000 * price.InputPrice
	out := float64(completionTokens) / 1_000_000 * price.OutputPrice
	currency := price.Currency
	if currency == "" {
		currency = billinginternal.DefaultPricingCurrency
	}
	return PricingBreakdown{
		TotalCost:  total,
		InputCost:  in,
		OutputCost: out,
		Currency:   currency,
		Source:     billinginternal.PriceSourceLegacyTable,
	}
}

// EstimateCostByType 按价格类型估算成本
func EstimateCostByType(backendID, model string, promptTokens, completionTokens int, priceType billingpkg.PriceType) PricingBreakdown {
	if promptTokens <= 0 && completionTokens <= 0 {
		return PricingBreakdown{Currency: billinginternal.DefaultPricingCurrency, Source: "zero", PriceType: priceType}
	}
	if pricingSvc != nil {
		bd, err := pricingSvc.EstimateCostByType(context.Background(), backendID, model, promptTokens, completionTokens, priceType)
		if err == nil && bd != nil {
			return PricingBreakdown{
				TotalCost:     bd.TotalCost,
				InputCost:     bd.InputCost,
				OutputCost:    bd.OutputCost,
				Currency:      bd.Currency,
				PricingRuleID: bd.PricingRuleID,
				Source:        bd.Source,
				PriceType:     priceType,
			}
		}
	}
	// 降级到默认估算
	return EstimateCostDetailed(backendID, model, promptTokens, completionTokens)
}

// DualPricingResult 包含成本和营收的价格估算
type DualPricingResult struct {
	CostBreakdown    PricingBreakdown
	RevenueBreakdown PricingBreakdown
}

// EstimateDualPricing 同时估算成本和营收价格
func EstimateDualPricing(backendID, model string, promptTokens, completionTokens int) DualPricingResult {
	costBD := EstimateCostByType(backendID, model, promptTokens, completionTokens, billingpkg.PriceTypeCost)
	revenueBD := EstimateCostByType(backendID, model, promptTokens, completionTokens, billingpkg.PriceTypeRevenue)
	return DualPricingResult{
		CostBreakdown:    costBD,
		RevenueBreakdown: revenueBD,
	}
}
