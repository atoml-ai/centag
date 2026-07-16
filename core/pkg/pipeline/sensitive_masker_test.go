package pipeline

import "testing"

func TestMaskSensitiveDataCoversPluginAuditFields(t *testing.T) {
	input := `api_key=sk-abcdefghijklmnopqrstuvwxyz password:supersecret token="abcdef123456" secrets_ref=my-secret`
	masked := MaskSensitiveData(input)

	for _, sensitive := range []string{
		"sk-abcdefghijklmnopqrstuvwxyz",
		"supersecret",
		"abcdef123456",
		"my-secret",
	} {
		if contains(masked, sensitive) {
			t.Fatalf("masked output still contains sensitive value %q: %s", sensitive, masked)
		}
	}
}
