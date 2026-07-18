package billing

import (
	"strings"
	"sync"
)

// DefaultUSDToCNY is used when pricing YAML omits usd_to_cny.
const DefaultUSDToCNY = 7.2

var exchangeRate = struct {
	mu       sync.RWMutex
	usdToCNY float64
}{usdToCNY: DefaultUSDToCNY}

// USDToCNY returns the current USD→CNY rate for display conversion.
func USDToCNY() float64 {
	exchangeRate.mu.RLock()
	defer exchangeRate.mu.RUnlock()
	if exchangeRate.usdToCNY <= 0 {
		return DefaultUSDToCNY
	}
	return exchangeRate.usdToCNY
}

// SetUSDToCNY updates the runtime display exchange rate.
func SetUSDToCNY(rate float64) {
	if rate <= 0 {
		rate = DefaultUSDToCNY
	}
	exchangeRate.mu.Lock()
	exchangeRate.usdToCNY = rate
	exchangeRate.mu.Unlock()
}

// ApplyPricingFileMeta applies currency/FX metadata from a pricing file to runtime.
func ApplyPricingFileMeta(file *PricingRulesFile) {
	if file == nil {
		return
	}
	if file.USDToCNY > 0 {
		SetUSDToCNY(file.USDToCNY)
	}
}

// NormalizePricingFileToUSD converts file contents to USD storage units.
// If the file (or a rule) is marked CNY, prices are divided by usd_to_cny.
// Always sets file.Currency to USD and updates the runtime FX rate.
func NormalizePricingFileToUSD(file *PricingRulesFile) {
	if file == nil {
		return
	}
	rate := file.USDToCNY
	if rate <= 0 {
		rate = USDToCNY()
	}
	fileCurrency := strings.ToUpper(strings.TrimSpace(file.Currency))
	if fileCurrency == "" {
		fileCurrency = DefaultPricingCurrency
	}

	for i := range file.Rules {
		r := &file.Rules[i]
		ruleCur := strings.ToUpper(strings.TrimSpace(r.Currency))
		if ruleCur == "" {
			ruleCur = fileCurrency
		}
		if ruleCur == "CNY" {
			r.InputPricePerM /= rate
			r.OutputPricePerM /= rate
		}
		r.Currency = DefaultPricingCurrency
	}

	file.Currency = DefaultPricingCurrency
	if file.USDToCNY <= 0 {
		file.USDToCNY = rate
	}
	SetUSDToCNY(file.USDToCNY)
}

// ConvertUSDToDisplay converts a USD amount to the requested display currency.
func ConvertUSDToDisplay(amountUSD float64, displayCurrency string) float64 {
	if strings.EqualFold(strings.TrimSpace(displayCurrency), "CNY") {
		return amountUSD * USDToCNY()
	}
	return amountUSD
}
