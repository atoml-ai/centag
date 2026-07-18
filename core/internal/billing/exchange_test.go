package billing

import "testing"

func TestNormalizePricingFileToUSD_FromCNY(t *testing.T) {
	SetUSDToCNY(DefaultUSDToCNY)
	file := &PricingRulesFile{
		Currency: "CNY",
		USDToCNY: 7.2,
		Rules: []PricingRule{
			{BackendID: "b", Model: "m", InputPricePerM: 7.2, OutputPricePerM: 14.4, Currency: "CNY"},
		},
	}
	NormalizePricingFileToUSD(file)
	if file.Currency != "USD" {
		t.Fatalf("currency %s", file.Currency)
	}
	if file.Rules[0].InputPricePerM != 1.0 || file.Rules[0].OutputPricePerM != 2.0 {
		t.Fatalf("prices %+v", file.Rules[0])
	}
	if file.Rules[0].Currency != "USD" {
		t.Fatalf("rule currency %s", file.Rules[0].Currency)
	}
	if USDToCNY() != 7.2 {
		t.Fatalf("fx %v", USDToCNY())
	}
}

func TestConvertUSDToDisplay(t *testing.T) {
	SetUSDToCNY(7.2)
	if got := ConvertUSDToDisplay(1.0, "CNY"); got != 7.2 {
		t.Fatalf("got %v", got)
	}
	if got := ConvertUSDToDisplay(1.0, "USD"); got != 1.0 {
		t.Fatalf("got %v", got)
	}
}
