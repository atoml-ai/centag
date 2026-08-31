package tokenusage

import (
	"context"

	billinginternal "centag/core/internal/billing"
	billingpkg "centag/core/pkg/billing"
)

var pricingSvc billinginternal.PricingService

// SetPricingService injects the billing PricingService used for cost attribution.
// Safe to call with nil to clear (tests / teardown).
func SetPricingService(s billinginternal.PricingService) {
	pricingSvc = s
}

// PricingBreakdown is the detailed estimate used when writing usage rows.
type PricingBreakdown struct {
	TotalCost       float64
	InputCost       float64
	OutputCost      float64
	Currency        string
	PricingRuleID   int64
	Source          string
	PriceType       billingpkg.PriceType
	InputPricePerM  float64 // USD per 1M tokens
	OutputPricePerM float64 // USD per 1M tokens
}

// EstimateCost computes prompt + completion cost via PricingService.
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
				TotalCost:       bd.TotalCost,
				InputCost:       bd.InputCost,
				OutputCost:      bd.OutputCost,
				Currency:        bd.Currency,
				PricingRuleID:   bd.PricingRuleID,
				Source:          bd.Source,
				PriceType:       bd.PriceType,
				InputPricePerM:  bd.InputPricePerM,
				OutputPricePerM: bd.OutputPricePerM,
			}
		}
	}
	// No pricing service configured — return zero cost.
	return PricingBreakdown{
		Currency: billinginternal.DefaultPricingCurrency,
		Source:   "no_pricing_service",
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
				TotalCost:       bd.TotalCost,
				InputCost:       bd.InputCost,
				OutputCost:      bd.OutputCost,
				Currency:        bd.Currency,
				PricingRuleID:   bd.PricingRuleID,
				Source:          bd.Source,
				PriceType:       priceType,
				InputPricePerM:  bd.InputPricePerM,
				OutputPricePerM: bd.OutputPricePerM,
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

func (b PricingBreakdown) toPkg() billingpkg.CostBreakdown {
	return billingpkg.CostBreakdown{
		InputCost:       b.InputCost,
		OutputCost:      b.OutputCost,
		TotalCost:       b.TotalCost,
		Currency:        b.Currency,
		PricingRuleID:   b.PricingRuleID,
		Source:          b.Source,
		PriceType:       b.PriceType,
		InputPricePerM:  b.InputPricePerM,
		OutputPricePerM: b.OutputPricePerM,
	}
}

func fromPkgBreakdown(b billingpkg.CostBreakdown) PricingBreakdown {
	return PricingBreakdown{
		TotalCost:       b.TotalCost,
		InputCost:       b.InputCost,
		OutputCost:      b.OutputCost,
		Currency:        b.Currency,
		PricingRuleID:   b.PricingRuleID,
		Source:          b.Source,
		PriceType:       b.PriceType,
		InputPricePerM:  b.InputPricePerM,
		OutputPricePerM: b.OutputPricePerM,
	}
}
