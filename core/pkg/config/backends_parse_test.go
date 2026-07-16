package config

import "testing"

func TestParseAdminBackendsJSON_AlternateAPIKeyKeys(t *testing.T) {
	t.Parallel()
	raw := `[{"id":"bigmodel","name":"n","type":"openai","base_url":"https://example/v1","APIKey":"sk-from-APIKey-field","enabled":true,"timeout":60,"max_retries":3}]`
	bl, err := ParseAdminBackendsJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(bl) != 1 || bl[0].APIKey != "sk-from-APIKey-field" {
		t.Fatalf("want APIKey merged, got %#v", bl)
	}
}
