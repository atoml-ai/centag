package configsync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Snapshot is the persisted sync state written to stateDir.
type Snapshot struct {
	Schema      int             `json:"schema"`
	GeneratedAt time.Time       `json:"generated_at"`
	Config      []Row           `json:"config"`
	Prices      []ProviderPrice `json:"prices,omitempty"`
}

const snapshotSchema = 1

const snapshotFileName = "configsync-snapshot.json"

// ReadSnapshot reads the last-good snapshot from stateDir.
// Returns (nil, nil) if no snapshot exists.
func ReadSnapshot(stateDir string) (*Snapshot, error) {
	path := filepath.Join(stateDir, snapshotFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read snapshot: %w", err)
	}
	var snap Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, fmt.Errorf("decode snapshot: %w", err)
	}
	return &snap, nil
}

// WriteSnapshot persists the sync state to stateDir with 0600 permissions.
func WriteSnapshot(stateDir string, snap *Snapshot) error {
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		return fmt.Errorf("mkdir stateDir: %w", err)
	}
	snap.Schema = snapshotSchema
	snap.GeneratedAt = time.Now()
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal snapshot: %w", err)
	}
	path := filepath.Join(stateDir, snapshotFileName)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write snapshot: %w", err)
	}
	return nil
}

// RateLimitError reports a 429 from the storage channel with the server
// requested retry delay. The scheduler honours it before the next attempt.
type RateLimitError struct {
	RetryAfter time.Duration
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("rate limited, retry after %s", e.RetryAfter)
}

// ConfigScheduler polls a Provider at configurable intervals with jitter,
// error backoff, batch validation and snapshot persistence. It is fail-open:
// startup never blocks, and errors keep the last-good snapshot in effect.
type ConfigScheduler struct {
	provider Provider
	stateDir string
	interval time.Duration
	onUpdate func(*Snapshot)

	mu           sync.Mutex
	snap         *Snapshot
	status       Status
	failCount    int
	lastRatelimit *RateLimitError
	syncing      bool

	stopCh  chan struct{}
	stopOne sync.Once
}

// SchedulerConfig configures the ConfigScheduler.
type SchedulerConfig struct {
	Provider Provider
	StateDir string
	Interval time.Duration // default 30m
	OnUpdate func(*Snapshot)
}

// NewScheduler creates a scheduler and immediately loads the last-good
// snapshot from stateDir if present (fail-open on startup).
func NewScheduler(cfg SchedulerConfig) *ConfigScheduler {
	if cfg.Interval == 0 {
		cfg.Interval = 30 * time.Minute
	}
	s := &ConfigScheduler{
		provider: cfg.Provider,
		stateDir: cfg.StateDir,
		interval: cfg.Interval,
		onUpdate: cfg.OnUpdate,
		stopCh:   make(chan struct{}),
	}
	if cfg.StateDir != "" {
		if snap, err := ReadSnapshot(cfg.StateDir); err == nil {
			s.snap = snap
		}
	}
	return s
}

// Start begins the polling loop and returns immediately (TC-SCH-001).
// The first sync is delayed by a 30-90s jitter (TC-SCH-002).
func (s *ConfigScheduler) Start(ctx context.Context) {
	go s.run(ctx)
}

// Stop terminates the polling loop exactly once.
func (s *ConfigScheduler) Stop() {
	s.stopOne.Do(func() { close(s.stopCh) })
}

// initialJitter is the randomized delay before the first pull (30-90s).
func initialJitter() time.Duration {
	return time.Duration(30+rand.Intn(61)) * time.Second
}

// nextDelay computes the delay before the next attempt: the jittered interval
// on success; exponential backoff on consecutive failures (capped at 8x
// interval); the server-requested window on 429 (TC-SCH-004/005).
func (s *ConfigScheduler) nextDelay() time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lastRatelimit != nil && s.lastRatelimit.RetryAfter > 0 {
		d := s.lastRatelimit.RetryAfter
		s.lastRatelimit = nil
		return d
	}
	base := float64(s.interval) * (0.9 + rand.Float64()*0.2) // ±10% jitter
	backoff := base * float64(uint64(1)<<min(s.failCount, 3))
	if backoff > base*8 {
		backoff = base * 8
	}
	return time.Duration(backoff)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Status returns the current sync status.
func (s *ConfigScheduler) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

// Snapshot returns the last-good snapshot (may be nil).
func (s *ConfigScheduler) Snapshot() *Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.snap
}

// SyncNow triggers one sync immediately. Concurrent calls are single-flight:
// while one sync is in flight others return without duplicating work
// (TC-SCH-008).
func (s *ConfigScheduler) SyncNow(ctx context.Context) error {
	s.mu.Lock()
	if s.syncing {
		s.mu.Unlock()
		return errors.New("sync already in flight")
	}
	s.syncing = true
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.syncing = false
		s.mu.Unlock()
	}()
	return s.doSync(ctx)
}

func (s *ConfigScheduler) run(ctx context.Context) {
	timer := time.NewTimer(initialJitter())
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return
	case <-s.stopCh:
		return
	case <-timer.C:
	}
	_ = s.SyncNow(ctx)
	for {
		timer.Reset(s.nextDelay())
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		case <-timer.C:
			_ = s.SyncNow(ctx)
		}
	}
}

// recordFailure records a failed attempt for backoff purposes.
func (s *ConfigScheduler) recordFailure(err error) {
	s.mu.Lock()
	s.status.LastSyncOK = false
	s.status.LastError = err.Error()
	s.status.ErrorCount++
	s.failCount++
	var rle *RateLimitError
	if errors.As(err, &rle) {
		s.lastRatelimit = rle
	}
	s.mu.Unlock()
}

// doSync fetches, validates and applies one round of remote data.
// A tampered/invalid batch is rejected wholesale, keeping last-good
// (TC-E2E-003 / TC-VAL-008). An entirely empty batch is a no-op success
// that does not overwrite the cache (TC-VAL-010).
func (s *ConfigScheduler) doSync(ctx context.Context) error {
	rows, err := s.provider.FetchConfig(ctx, Query{})
	if err != nil {
		s.recordFailure(err)
		return err
	}
	if err := ValidateRows(rows); err != nil {
		err = fmt.Errorf("invalid batch rejected: %w", err)
		s.recordFailure(err)
		return err
	}
	prices, err := s.provider.FetchModelPrices(ctx)
	if err != nil && !errors.Is(err, ErrNotSupported) {
		s.recordFailure(err)
		return err
	}
	for i := range prices {
		if err := ValidatePriceRow(&prices[i]); err != nil {
			err = fmt.Errorf("invalid price batch rejected: %w", err)
			s.recordFailure(err)
			return err
		}
	}
	// Empty batch: keep cache, count as success (nothing to do).
	if len(rows) == 0 && len(prices) == 0 {
		s.mu.Lock()
		s.status.LastSyncTime = time.Now()
		s.status.LastSyncOK = true
		s.status.LastError = ""
		s.status.SyncCount++
		s.failCount = 0
		s.mu.Unlock()
		return nil
	}
	snap := &Snapshot{Schema: snapshotSchema, GeneratedAt: time.Now(), Config: rows, Prices: prices}
	if s.stateDir != "" {
		_ = WriteSnapshot(s.stateDir, snap)
	}
	s.mu.Lock()
	s.snap = snap
	s.status.LastSyncTime = time.Now()
	s.status.LastSyncOK = true
	s.status.LastError = ""
	s.status.SyncCount++
	s.failCount = 0
	s.mu.Unlock()
	if s.onUpdate != nil {
		s.onUpdate(snap)
	}
	return nil
}
