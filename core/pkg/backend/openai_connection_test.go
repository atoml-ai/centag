package backend

import (
	"strings"
	"testing"
)

func TestKimiMembershipOrPaymentError_KimiCodingEndpoint(t *testing.T) {
	cfg := &BackendConfig{BaseURL: "https://api.kimi.com/coding/v1"}
	body := []byte(`{"error":{"message":"We're unable to verify your membership benefits at this time.","type":"invalid_request_error"}}`)

	err := kimiMembershipOrPaymentError(cfg, "https://api.kimi.com/coding/v1/models", body)
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "HTTP 402") {
		t.Fatalf("expected HTTP 402 in message, got: %s", msg)
	}
	if !strings.Contains(msg, "Kimi for Coding") {
		t.Fatalf("expected Kimi for Coding hint, got: %s", msg)
	}
	if !strings.Contains(msg, "api.moonshot.cn") {
		t.Fatalf("expected moonshot URL hint, got: %s", msg)
	}
}

func TestKimiMembershipOrPaymentError_GenericPayment(t *testing.T) {
	cfg := &BackendConfig{BaseURL: "https://api.example.com/v1"}
	body := []byte(`{"error":{"message":"payment required"}}`)

	err := kimiMembershipOrPaymentError(cfg, "https://api.example.com/v1/chat/completions", body)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "账户权益或余额不足") {
		t.Fatalf("unexpected message: %s", err.Error())
	}
}