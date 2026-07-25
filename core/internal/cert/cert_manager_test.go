package cert

import (
	"crypto/tls"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"centag/core/pkg/logger"
)

func TestMain(m *testing.M) {
	_ = logger.Init(logger.Config{Level: "error", Format: "console", Output: "stdout"})
	os.Exit(m.Run())
}

func newTestCertManager(t *testing.T) *CertManager {
	t.Helper()
	dir := t.TempDir()
	m, err := NewCertManager(
		filepath.Join(dir, "ca.crt"),
		filepath.Join(dir, "ca.key"),
		dir,
		90,
	)
	if err != nil {
		t.Fatalf("NewCertManager: %v", err)
	}
	m.ClearCache()
	// GenerateCertForDomain 异步写 .crt/.key，避免 TempDir 清理时目录仍被写入
	t.Cleanup(func() {
		time.Sleep(300 * time.Millisecond)
	})
	return m
}

func TestGetOrCreateTLSCertificate_CacheHit(t *testing.T) {
	m := newTestCertManager(t)

	c1, err := m.GetOrCreateTLSCertificate("api.example.com")
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	c2, err := m.GetOrCreateTLSCertificate("api.example.com")
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if c1 != c2 {
		t.Fatalf("expected same cached *tls.Certificate pointer")
	}
	if len(c1.Certificate) == 0 || c1.PrivateKey == nil {
		t.Fatal("certificate incomplete")
	}
}

func TestGetOrCreateTLSCertificate_NormalizesDomain(t *testing.T) {
	m := newTestCertManager(t)

	c1, err := m.GetOrCreateTLSCertificate("API.Example.COM")
	if err != nil {
		t.Fatalf("upper: %v", err)
	}
	c2, err := m.GetOrCreateTLSCertificate("api.example.com")
	if err != nil {
		t.Fatalf("lower: %v", err)
	}
	if c1 != c2 {
		t.Fatalf("domain case should share cache entry")
	}
}

func TestGetOrCreateTLSCertificate_ConcurrentSingleflight(t *testing.T) {
	m := newTestCertManager(t)

	const n = 16
	certs := make([]*tls.Certificate, n)
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		i := i
		go func() {
			defer wg.Done()
			c, err := m.GetOrCreateTLSCertificate("concurrent.example.com")
			if err != nil {
				t.Errorf("goroutine %d: %v", i, err)
				return
			}
			certs[i] = c
		}()
	}
	wg.Wait()

	var nonNil atomic.Int32
	var first *tls.Certificate
	for _, c := range certs {
		if c == nil {
			continue
		}
		nonNil.Add(1)
		if first == nil {
			first = c
			continue
		}
		if c != first {
			t.Fatalf("concurrent calls returned different certificate pointers (singleflight failed)")
		}
	}
	if nonNil.Load() != n {
		t.Fatalf("got %d certs, want %d", nonNil.Load(), n)
	}
}

func TestClearCache_DropsTLSEntries(t *testing.T) {
	m := newTestCertManager(t)
	c1, err := m.GetOrCreateTLSCertificate("clear.example.com")
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	m.ClearCache()
	c2, err := m.GetOrCreateTLSCertificate("clear.example.com")
	if err != nil {
		t.Fatalf("after clear: %v", err)
	}
	if c1 == c2 {
		t.Fatal("expected new certificate after ClearCache")
	}
}
