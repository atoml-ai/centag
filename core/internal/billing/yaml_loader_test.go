package billing

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParsePricingYAML_DefaultsEnabled(t *testing.T) {
	raw := []byte(`
version: "1.0"
currency: "USD"
usd_to_cny: 7.2
rules:
  - name: "x"
    backend_id: "b"
    model: "m"
    input_price_per_m: 1
    output_price_per_m: 2
`)
	file, err := ParsePricingYAML(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(file.Rules) != 1 || !file.Rules[0].Enabled {
		t.Fatalf("expected enabled=true by default, got %+v", file.Rules)
	}
	if file.USDToCNY != 7.2 {
		t.Fatalf("usd_to_cny %v", file.USDToCNY)
	}
}

func TestParsePricingYAML_ExplicitDisabled(t *testing.T) {
	raw := []byte(`
version: "1.0"
currency: "USD"
rules:
  - name: "x"
    backend_id: "b"
    model: "m"
    input_price_per_m: 1
    output_price_per_m: 2
    enabled: false
`)
	file, err := ParsePricingYAML(raw)
	if err != nil {
		t.Fatal(err)
	}
	if file.Rules[0].Enabled {
		t.Fatal("expected enabled=false")
	}
}

func TestYAMLRoundTrip(t *testing.T) {
	candidates := []string{
		filepath.Join("..", "..", "..", "config", "pricing", "default.yaml"),
		"config/pricing/default.yaml",
	}
	var data []byte
	var err error
	for _, c := range candidates {
		data, err = os.ReadFile(c)
		if err == nil {
			break
		}
	}
	if err != nil {
		t.Skip("default.yaml not found from test cwd")
	}
	file, err := ParsePricingYAML(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(file.Rules) < 5 {
		t.Fatalf("expected seed rules, got %d", len(file.Rules))
	}
	out, err := MarshalPricingYAML(file)
	if err != nil {
		t.Fatal(err)
	}
	again, err := ParsePricingYAML(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(again.Rules) != len(file.Rules) {
		t.Fatalf("roundtrip len %d vs %d", len(again.Rules), len(file.Rules))
	}
}

func TestYAMLParsing(t *testing.T) {
	TestParsePricingYAML_DefaultsEnabled(t)
}
