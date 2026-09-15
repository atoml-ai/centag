package mitm

import (
	"testing"
	"time"
)

func TestFailureSignals_RecordAndCheck(t *testing.T) {
	fs := NewFailureSignals(3, 5*time.Minute)

	// Record 2 TLS handshake failures - should not trigger
	fs.Record(FailureTLSHandshake, "api.openai.com", "tls: unknown authority")
	fs.Record(FailureTLSHandshake, "api.openai.com", "tls: unknown authority")
	if fs.CheckCertPinning("api.openai.com") {
		t.Fatal("2 failures should not trigger threshold of 3")
	}

	// Record 3rd failure - should trigger
	fs.Record(FailureTLSHandshake, "api.openai.com", "tls: unknown authority")
	if !fs.CheckCertPinning("api.openai.com") {
		t.Fatal("3 failures should trigger threshold of 3")
	}

	// Different domain should not trigger
	if fs.CheckCertPinning("other.domain.com") {
		t.Fatal("different domain should not trigger")
	}
}

func TestFailureSignals_WindowExpiry(t *testing.T) {
	fs := NewFailureSignals(2, 1*time.Millisecond) // Very short window

	// Record failures
	fs.Record(FailureTLSHandshake, "api.openai.com", "error1")
	fs.Record(FailureTLSHandshake, "api.openai.com", "error2")

	// Wait for window to expire
	time.Sleep(5 * time.Millisecond)

	// Should not trigger due to window expiry
	if fs.CheckCertPinning("api.openai.com") {
		t.Fatal("failures outside window should not trigger")
	}
}

func TestFailureSignals_GetDomainStats(t *testing.T) {
	fs := NewFailureSignals(5, 5*time.Minute)

	fs.Record(FailureTLSHandshake, "api.openai.com", "error1")
	fs.Record(FailureTLSHandshake, "api.openai.com", "error2")
	fs.Record(FailureClientReset, "api.openai.com", "error3")
	fs.Record(FailureTLSHandshake, "other.com", "error4")

	tlsFails, totalFails := fs.GetDomainStats("api.openai.com")
	if tlsFails != 2 {
		t.Fatalf("expected 2 TLS failures, got %d", tlsFails)
	}
	if totalFails != 3 {
		t.Fatalf("expected 3 total failures, got %d", totalFails)
	}
}

func TestFailureSignals_GetRecentSignals(t *testing.T) {
	fs := NewFailureSignals(5, 5*time.Minute)

	fs.Record(FailureTLSHandshake, "domain1.com", "error1")
	fs.Record(FailureClientReset, "domain2.com", "error2")
	fs.Record(FailureTLSHandshake, "domain3.com", "error3")

	signals := fs.GetRecentSignals(2)
	if len(signals) != 2 {
		t.Fatalf("expected 2 signals, got %d", len(signals))
	}
	if signals[0].Domain != "domain2.com" {
		t.Fatalf("expected domain2.com, got %s", signals[0].Domain)
	}
	if signals[1].Domain != "domain3.com" {
		t.Fatalf("expected domain3.com, got %s", signals[1].Domain)
	}
}

func TestFailureSignals_Clear(t *testing.T) {
	fs := NewFailureSignals(5, 5*time.Minute)

	fs.Record(FailureTLSHandshake, "api.openai.com", "error1")
	fs.Record(FailureTLSHandshake, "api.openai.com", "error2")

	fs.Clear()

	if fs.CheckCertPinning("api.openai.com") {
		t.Fatal("cleared signals should not trigger")
	}
}

func TestFailureSignals_TrimOldSignals(t *testing.T) {
	fs := NewFailureSignals(5, 5*time.Minute)
	fs.maxSize = 5 // Small size for testing

	// Add more than maxSize
	for i := 0; i < 10; i++ {
		fs.Record(FailureTLSHandshake, "api.openai.com", "error")
	}

	fs.mu.RLock()
	size := len(fs.signals)
	fs.mu.RUnlock()

	if size > 5 {
		t.Fatalf("expected max 5 signals, got %d", size)
	}
}
