package tokenusage

import "centag/core/pkg/scheduler"

// priceTable provides model pricing for cost attribution.
// Values follow the currency configured per model in the price table (often CNY).
var priceTable = scheduler.NewModelPriceTable()

// EstimateCost computes prompt + completion cost using the shared model price table.
func EstimateCost(backendID, model string, promptTokens, completionTokens int) float64 {
	if promptTokens <= 0 && completionTokens <= 0 {
		return 0
	}
	return priceTable.EstimateCost(backendID, model, promptTokens, completionTokens)
}