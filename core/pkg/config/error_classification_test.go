package config

import "testing"

func TestIsBillingOrQuotaFailure(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		want   bool
	}{
		{"402", 402, "whatever", true},
		{"401 credits", 401, `{"error":{"type":"CreditsError","message":"Insufficient credits"}}`, true},
		{"401 plain auth", 401, `{"error":"invalid_api_key"}`, false},
		{"msg only", 0, "not_enough_balance", true},
		{"quota", 403, "insufficient_quota", true},
		{"empty", 401, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsBillingOrQuotaFailure(tc.status, tc.body); got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}

func TestIsRetryableError_Billing(t *testing.T) {
	// billing/quota 失败是永久性失败：上游账户余额耗尽不会自愈，engine 层
	// 不应再重试（避免 N×M 倍请求放大）。降级交由 transparent 节点自身或
	// pipeline 显式声明的 FallbackGroups 处理。
	if IsRetryableError("billing", 0, "") {
		t.Fatal("billing should NOT be retryable at engine layer (avoids N×M amplification)")
	}
	if !IsRetryableError("http_status", 401, "CreditsError") {
		t.Fatal("401+CreditsError should be degradable")
	}
	if IsRetryableError("http_status", 401, "invalid_api_key") {
		t.Fatal("plain 401 auth should not be retryable")
	}
}
