package mitm

import (
	"sync"
	"time"
)

// FailureSignalType represents the type of MITM failure.
type FailureSignalType string

const (
	// FailureTLSHandshake indicates TLS handshake failure (possible certificate pinning).
	FailureTLSHandshake FailureSignalType = "tls_handshake"
	// FailureClientReset indicates client connection reset.
	FailureClientReset FailureSignalType = "client_reset"
	// FailureUnknownAuthority indicates unknown certificate authority.
	FailureUnknownAuthority FailureSignalType = "unknown_authority"
	// FailureConnectionRefused indicates connection refused.
	FailureConnectionRefused FailureSignalType = "connection_refused"
)

// FailureSignal records a single MITM failure event.
type FailureSignal struct {
	Type      FailureSignalType
	Domain    string
	Timestamp time.Time
	Error     string
}

// FailureSignals collects MITM failure signals for certificate pinning detection.
type FailureSignals struct {
	mu          sync.RWMutex
	signals     []FailureSignal
	maxSize     int
	threshold   int           // number of failures to trigger warning
	window      time.Duration // time window for threshold
}

// NewFailureSignals creates a new FailureSignals collector.
func NewFailureSignals(threshold int, window time.Duration) *FailureSignals {
	return &FailureSignals{
		signals:   make([]FailureSignal, 0, 128),
		maxSize:   1024,
		threshold: threshold,
		window:    window,
	}
}

// Record adds a failure signal to the collector.
func (fs *FailureSignals) Record(sigType FailureSignalType, domain, errMsg string) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	fs.signals = append(fs.signals, FailureSignal{
		Type:      sigType,
		Domain:    domain,
		Timestamp: time.Now(),
		Error:     errMsg,
	})

	// Trim old signals if we exceed max size
	if len(fs.signals) > fs.maxSize {
		fs.signals = fs.signals[len(fs.signals)-fs.maxSize:]
	}
}

// CheckCertPinning checks if there are signs of certificate pinning for a domain.
// Returns true if the threshold is exceeded within the time window.
func (fs *FailureSignals) CheckCertPinning(domain string) bool {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	cutoff := time.Now().Add(-fs.window)
	count := 0
	for _, sig := range fs.signals {
		if sig.Domain == domain && sig.Timestamp.After(cutoff) && sig.Type == FailureTLSHandshake {
			count++
		}
	}
	return count >= fs.threshold
}

// GetDomainStats returns failure statistics for a domain.
func (fs *FailureSignals) GetDomainStats(domain string) (tlsFailures, totalFailures int) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	cutoff := time.Now().Add(-fs.window)
	for _, sig := range fs.signals {
		if sig.Domain == domain && sig.Timestamp.After(cutoff) {
			totalFailures++
			if sig.Type == FailureTLSHandshake {
				tlsFailures++
			}
		}
	}
	return
}

// GetRecentSignals returns recent failure signals (for diagnostic output).
func (fs *FailureSignals) GetRecentSignals(limit int) []FailureSignal {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	if limit <= 0 || limit > len(fs.signals) {
		limit = len(fs.signals)
	}
	result := make([]FailureSignal, limit)
	copy(result, fs.signals[len(fs.signals)-limit:])
	return result
}

// Clear removes all recorded signals.
func (fs *FailureSignals) Clear() {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	fs.signals = fs.signals[:0]
}
