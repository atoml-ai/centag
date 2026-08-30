package configsync

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ValidateConfigRow validates a single config row for structural correctness.
// Returns an error if the row is invalid.
func ValidateConfigRow(row *Row) error {
	if row.Key == "" {
		return fmt.Errorf("config_key is empty")
	}
	if row.Edition == "" {
		return fmt.Errorf("edition is empty for key %q", row.Key)
	}
	switch row.Edition {
	case "all", "personal", "team":
		// ok
	default:
		return fmt.Errorf("invalid edition %q for key %q", row.Edition, row.Key)
	}
	if row.Channel != "" {
		switch row.Channel {
		case "stable", "beta", "dev":
			// ok
		default:
			return fmt.Errorf("invalid channel %q for key %q", row.Channel, row.Key)
		}
	}
	if row.MinVersion != "" && row.MaxVersion != "" {
		if err := ValidateSemver(row.MinVersion); err != nil {
			return fmt.Errorf("min_version %q invalid for key %q: %w", row.MinVersion, row.Key, err)
		}
		if err := ValidateSemver(row.MaxVersion); err != nil {
			return fmt.Errorf("max_version %q invalid for key %q: %w", row.MaxVersion, row.Key, err)
		}
		if CompareVersions(row.MinVersion, row.MaxVersion) > 0 {
			return fmt.Errorf("min_version %q > max_version %q for key %q", row.MinVersion, row.MaxVersion, row.Key)
		}
	} else if row.MinVersion != "" {
		if err := ValidateSemver(row.MinVersion); err != nil {
			return fmt.Errorf("min_version %q invalid for key %q: %w", row.MinVersion, row.Key, err)
		}
	} else if row.MaxVersion != "" {
		if err := ValidateSemver(row.MaxVersion); err != nil {
			return fmt.Errorf("max_version %q invalid for key %q: %w", row.MaxVersion, row.Key, err)
		}
	}
	if len(row.Value) > 1<<20 { // 1 MiB
		return fmt.Errorf("value too large (%d bytes) for key %q", len(row.Value), row.Key)
	}
	if len(row.Value) > 0 {
		var raw json.RawMessage
		if err := json.Unmarshal(row.Value, &raw); err != nil {
			return fmt.Errorf("value is not valid JSON for key %q: %w", row.Key, err)
		}
		// Namespace-specific value schema (TC-VAL-006).
		switch {
		case strings.HasPrefix(row.Key, "release."):
			var vi VersionInfo
			if err := json.Unmarshal(row.Value, &vi); err != nil || strings.TrimSpace(vi.Version) == "" {
				return fmt.Errorf("release.* value must contain non-empty version for key %q", row.Key)
			}
		case strings.HasPrefix(row.Key, "table."):
			var obj map[string]any
			if err := json.Unmarshal(row.Value, &obj); err != nil || obj == nil {
				return fmt.Errorf("table.* value must be a JSON object for key %q", row.Key)
			}
		}
	}
	return nil
}

// ValidatePriceRow validates a single model price row.
func ValidatePriceRow(row *ProviderPrice) error {
	if row.BaseURL == "" {
		return fmt.Errorf("base_url is empty")
	}
	// Normalize trailing slash for comparison.
	row.BaseURL = strings.TrimRight(row.BaseURL, "/")
	if row.ProviderName == "" {
		return fmt.Errorf("provider_name is empty for base_url %q", row.BaseURL)
	}
	if row.Currency == "" {
		row.Currency = "USD"
	}
	if row.Currency != "USD" && row.Currency != "CNY" {
		return fmt.Errorf("invalid currency %q for base_url %q", row.Currency, row.BaseURL)
	}
	for _, m := range row.Models {
		if m.Model == "" {
			return fmt.Errorf("model name empty in base_url %q", row.BaseURL)
		}
		// 下界含 0：免费档/漏填价格一律拒收，防误同步为 0 成本（TC-VAL-002）。
		if m.InputPricePerM <= 0 || m.InputPricePerM > 10000 {
			return fmt.Errorf("input_price_per_m out of range for model %q in base_url %q", m.Model, row.BaseURL)
		}
		if m.OutputPricePerM <= 0 || m.OutputPricePerM > 10000 {
			return fmt.Errorf("output_price_per_m out of range for model %q in base_url %q", m.Model, row.BaseURL)
		}
	}
	return nil
}

// ValidateRows validates a batch of config rows. If any row is invalid,
// returns the first error. Empty batches are valid (no overwrite).
func ValidateRows(rows []Row) error {
	for i := range rows {
		if err := ValidateConfigRow(&rows[i]); err != nil {
			return fmt.Errorf("row %d: %w", i, err)
		}
	}
	return nil
}
