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
	"sync/atomic"
	"time"

	"centag/core/pkg/logger"
)

// globalScheduler holds the global scheduler instance for access from other packages.
var (
	globalScheduler   *ConfigScheduler
	globalSchedulerMu sync.RWMutex
)

// globalProviderCatalogStore holds the global provider catalog store for API access.
var (
	globalProviderCatalogStore   ProviderCatalogStore
	globalProviderCatalogStoreMu sync.RWMutex
)

// SetGlobalProviderCatalogStore sets the global provider catalog store.
func SetGlobalProviderCatalogStore(store ProviderCatalogStore) {
	globalProviderCatalogStoreMu.Lock()
	defer globalProviderCatalogStoreMu.Unlock()
	globalProviderCatalogStore = store
}

// GetGlobalProviderCatalogStore returns the global provider catalog store (may be nil).
func GetGlobalProviderCatalogStore() ProviderCatalogStore {
	globalProviderCatalogStoreMu.RLock()
	defer globalProviderCatalogStoreMu.RUnlock()
	return globalProviderCatalogStore
}

// SetGlobalScheduler sets the global scheduler instance.
func SetGlobalScheduler(s *ConfigScheduler) {
	globalSchedulerMu.Lock()
	defer globalSchedulerMu.Unlock()
	globalScheduler = s
}

// GetScheduler returns the global scheduler instance (may be nil).
func GetScheduler() *ConfigScheduler {
	globalSchedulerMu.RLock()
	defer globalSchedulerMu.RUnlock()
	return globalScheduler
}

// Snapshot is the persisted sync state written to stateDir.
type Snapshot struct {
	Schema            int                `json:"schema"`
	GeneratedAt       time.Time          `json:"generated_at"`
	Config            []Row              `json:"config"`
	Prices            []ProviderPrice    `json:"prices,omitempty"`
	PipelineTemplates []PipelineTemplate `json:"pipeline_templates,omitempty"`
	Skills            []RemoteSkillRow   `json:"skills,omitempty"`
	// SkillsFetched records that the provider implemented and successfully
	// returned the skill table (possibly empty). An explicit empty table means
	// "remote has no skills → clear local"; an absent/failed table keeps
	// last-good (§3.3 Step 2).
	SkillsFetched bool `json:"skills_fetched,omitempty"`

	// Tables holds generic table data keyed by logical table name (§4.4).
	// New table types (MITM domains, plan templates, system_config KV, etc.)
	// go here instead of adding dedicated top-level fields.
	Tables map[string]json.RawMessage `json:"tables,omitempty"`
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
	runOnce  bool

	mu            sync.Mutex
	snap          atomic.Pointer[Snapshot] // 读侧无锁；写侧构建新对象后单次 Store
	status        Status
	failCount     int
	lastRatelimit *RateLimitError
	syncing       bool

	stopCh  chan struct{}
	stopOne sync.Once
}

// Current returns the latest Snapshot (read-lock-free via atomic load).
// The returned Snapshot is read-only; consumers must not mutate it.
func (s *ConfigScheduler) Current() *Snapshot {
	return s.snap.Load()
}

// SchedulerConfig configures the ConfigScheduler.
type SchedulerConfig struct {
	Provider Provider
	StateDir string
	Interval time.Duration // default 30m; ignored when RunOnce=true
	OnUpdate func(*Snapshot)
	// RunOnce makes the scheduler sync exactly once on startup (no polling).
	// Subsequent syncs are triggered manually via SyncNow().
	RunOnce bool
}

// NewScheduler creates a scheduler and immediately loads the last-good
// snapshot from stateDir if present (fail-open on startup).
func NewScheduler(cfg SchedulerConfig) *ConfigScheduler {
	if cfg.Interval == 0 && !cfg.RunOnce {
		cfg.Interval = 30 * time.Minute
	}
	s := &ConfigScheduler{
		provider: cfg.Provider,
		stateDir: cfg.StateDir,
		interval: cfg.Interval,
		onUpdate: cfg.OnUpdate,
		runOnce:  cfg.RunOnce,
		stopCh:   make(chan struct{}),
	}
	if cfg.StateDir != "" {
		if snap, err := ReadSnapshot(cfg.StateDir); err == nil {
			s.snap.Store(snap)
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
	return s.snap.Load()
}

// SyncNow triggers one sync immediately. Concurrent calls are single-flight:
// while one sync is in flight others return "sync already in flight" without
// duplicating work or counting as a failure (TC-SCH-008).
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

// StartSkillPoll runs a periodic skill-only fetch on the same scheduler and
// provider, honouring the shared stopCh (so Stop() ends it too). This replaces
// the former standalone startFeishuSkillSync goroutine: skills stay on the
// unified config-sync path and share the CENTAG_CONFIGSYNC=off switch (§3.3).
// The first poll runs immediately; when interval <= 0 it defaults to 10m.
func (s *ConfigScheduler) StartSkillPoll(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 10 * time.Minute
	}
	go func() {
		_ = s.SyncSkillsNow(ctx)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-s.stopCh:
				return
			case <-ticker.C:
				_ = s.SyncSkillsNow(ctx)
			}
		}
	}()
}

// SyncSkillsNow fetches only the remote skill table and delivers a minimal
// snapshot to onUpdate. It never touches config/prices/etc. so it cannot
// overwrite the last-good config snapshot. An explicit empty remote table is
// surfaced with SkillsFetched=true so the consumer can clear local skills.
func (s *ConfigScheduler) SyncSkillsNow(ctx context.Context) error {
	type fetchSkillRows interface {
		FetchSkillRows(ctx context.Context) ([]RemoteSkillRow, error)
	}
	fsr, ok := s.provider.(fetchSkillRows)
	if !ok {
		return nil
	}
	skills, err := fsr.FetchSkillRows(ctx)
	if errors.Is(err, ErrNotSupported) {
		return nil
	}
	if err != nil {
		logger.Warnf("configsync: skill poll failed, keeping last-good: %v", err)
		s.recordFailure(err)
		return err
	}
	if s.onUpdate != nil {
		s.onUpdate(&Snapshot{
			Schema:        snapshotSchema,
			GeneratedAt:   time.Now(),
			Skills:        skills,
			SkillsFetched: true,
		})
	}
	s.mu.Lock()
	s.status.LastSyncTime = time.Now()
	s.status.LastFetchOK = true
	// Skill poll does not persist/apply the full snapshot, so it must not
	// advertise persist/apply success for the main config path.
	s.status.LastSyncOK = true
	s.status.LastError = ""
	s.mu.Unlock()
	return nil
}

func (s *ConfigScheduler) run(ctx context.Context) {
	// RunOnce mode: sync immediately and return (no polling).
	if s.runOnce {
		_ = s.SyncNow(ctx)
		return
	}
	// Normal polling mode.
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
	s.status.LastFetchOK = false
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
	type fetchAller interface {
		FetchAll(ctx context.Context, q Query) ([]Row, []ProviderPrice, error)
	}

	type fetchPipelineTemplates interface {
		FetchPipelineTemplates(ctx context.Context) ([]PipelineTemplate, error)
	}

	// fetchBackendRows is an optional provider extension that surfaces
	// backend configs as "backend.*" config rows (Feishu backend table).
	type fetchBackendRows interface {
		FetchBackendRows(ctx context.Context) ([]Row, error)
	}

	// fetchSkillRows is an optional provider extension that surfaces
	// remote agent skill rows (Feishu skill table).
	type fetchSkillRows interface {
		FetchSkillRows(ctx context.Context) ([]RemoteSkillRow, error)
	}

	// fetchAgentAppRows is an optional provider extension that surfaces
	// remote agent app catalog rows (Feishu agent_apps table, §5.4).
	type fetchAgentAppRows interface {
		FetchAgentAppRows(ctx context.Context) ([]AgentAppRow, error)
	}

	// fetchSystemConfigRows is an optional provider extension that surfaces
	// remote system_config KV rows (§5.4 P0-3).
	type fetchSystemConfigRows interface {
		FetchSystemConfigRows(ctx context.Context) ([]KVRow, error)
	}

	// fetchMITMDomainsRows is an optional provider extension that surfaces
	// remote MITM domain rows (§5.4 P0-3).
	type fetchMITMDomainsRows interface {
		FetchMITMDomainsRows(ctx context.Context) ([]MITMDomainRow, error)
	}

	var rows []Row
	var prices []ProviderPrice
	var pipelineTemplates []PipelineTemplate
	var skills []RemoteSkillRow
	var skillsFetched bool
	var agentApps []AgentAppRow
	var systemConfig []KVRow
	var mitmDomains []MITMDomainRow
	// mitmFetched records that the provider implemented and successfully
	// returned the mitm_domains table (possibly empty). Used to distinguish an
	// explicit empty table ("delete all") from an absent/failed table (keep
	// last-good) — §3.2 Step 3 P0-4.
	var mitmFetched bool
	var err error

	// Build query with edition and version from environment
	q := Query{
		Edition: os.Getenv("CENTAG_EDITION"),
		Version: os.Getenv("CENTAG_VERSION"),
		Channel: os.Getenv("CENTAG_CHANNEL"),
	}

	if fa, ok := s.provider.(fetchAller); ok {
		rows, prices, err = fa.FetchAll(ctx, q)
	} else {
		rows, err = s.provider.FetchConfig(ctx, q)
		if err != nil {
			s.recordFailure(err)
			return err
		}
		prices, err = s.provider.FetchModelPrices(ctx)
		if err != nil && !errors.Is(err, ErrNotSupported) {
			logger.Warnf("configsync: fetch model prices failed, continuing without prices: %v", err)
			prices = nil
			err = nil
		}
	}
	if err != nil && !errors.Is(err, ErrNotSupported) {
		s.recordFailure(err)
		return err
	}
	// Fetch pipeline templates if provider supports it
	if fpt, ok := s.provider.(fetchPipelineTemplates); ok {
		pipelineTemplates, err = fpt.FetchPipelineTemplates(ctx)
		if err != nil && !errors.Is(err, ErrNotSupported) {
			logger.Warnf("configsync: fetch pipeline templates failed: %v", err)
			// Continue with other data, don't fail the sync
		}
	}
	// Fetch backend config rows if provider supports it; they are merged into
	// rows so BackendApplier / DBConfigApplier / validation treat them like
	// any other "backend.*" config row.
	if fbr, ok := s.provider.(fetchBackendRows); ok {
		backendRows, berr := fbr.FetchBackendRows(ctx)
		if berr != nil && !errors.Is(berr, ErrNotSupported) {
			logger.Warnf("configsync: fetch backend configs failed: %v", berr)
			// Continue with other data, don't fail the sync
		} else if len(backendRows) > 0 {
			rows = append(rows, backendRows...)
		}
	}
	// Fetch remote agent skill rows if provider supports it (R13).
	if fsr, ok := s.provider.(fetchSkillRows); ok {
		fetchedSkills, sErr := fsr.FetchSkillRows(ctx)
		switch {
		case sErr == nil:
			skills = fetchedSkills
			skillsFetched = true
		case errors.Is(sErr, ErrNotSupported):
			// Provider has no skill channel → treat as absent (keep last-good).
		default:
			logger.Warnf("configsync: fetch skill rows failed, keeping last-good: %v", sErr)
		}
	}
	// Fetch remote agent app catalog rows if provider supports it (M3 §5.4).
	if faa, ok := s.provider.(fetchAgentAppRows); ok {
		fetchedApps, aaErr := faa.FetchAgentAppRows(ctx)
		if aaErr != nil && !errors.Is(aaErr, ErrNotSupported) {
			logger.Warnf("configsync: fetch agent app rows failed, continuing without agent apps: %v", aaErr)
		} else {
			agentApps = fetchedApps
		}
	}
	// Fetch remote system_config KV rows if provider supports it (P0-3 §5.4).
	if fsc, ok := s.provider.(fetchSystemConfigRows); ok {
		fetchedSC, scErr := fsc.FetchSystemConfigRows(ctx)
		if scErr != nil && !errors.Is(scErr, ErrNotSupported) {
			logger.Warnf("configsync: fetch system_config rows failed, continuing without system_config: %v", scErr)
		} else {
			systemConfig = fetchedSC
		}
	}
	// Fetch remote MITM domain rows if provider supports it (P0-3 §5.4).
	if fmd, ok := s.provider.(fetchMITMDomainsRows); ok {
		fetchedMD, mdErr := fmd.FetchMITMDomainsRows(ctx)
		switch {
		case mdErr == nil:
			mitmDomains = fetchedMD
			mitmFetched = true
		case errors.Is(mdErr, ErrNotSupported):
			// Provider has no mitm_domains channel → treat as absent.
		default:
			logger.Warnf("configsync: fetch mitm_domains rows failed, keeping last-good: %v", mdErr)
		}
	}
	if err := ValidateRows(rows); err != nil {
		err = fmt.Errorf("invalid batch rejected: %w", err)
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
	// Empty batch: keep cache, count as success (nothing to do). Nothing was
	// fetched that needs persisting or applying, but the fetch itself (an empty
	// successful response) is recorded as OK.
	if len(rows) == 0 && len(prices) == 0 && len(pipelineTemplates) == 0 && len(skills) == 0 && len(agentApps) == 0 && len(systemConfig) == 0 && len(mitmDomains) == 0 && !mitmFetched && !skillsFetched {
		s.mu.Lock()
		s.status.LastSyncTime = time.Now()
		s.status.LastSyncOK = true
		s.status.LastError = ""
		s.status.LastFetchOK = true
		s.status.LastPersistOK = true
		s.status.LastApplyOK = false
		s.status.SyncCount++
		s.failCount = 0
		s.mu.Unlock()
		return nil
	}
	snap := &Snapshot{
		Schema:            snapshotSchema,
		GeneratedAt:       time.Now(),
		Config:            rows,
		Prices:            prices,
		PipelineTemplates: pipelineTemplates,
		Skills:            skills,
		SkillsFetched:     skillsFetched,
	}
	// Serialize agent apps into Tables["agent_apps"] (§5.4)
	if len(agentApps) > 0 {
		if data, err := json.Marshal(agentApps); err == nil {
			if snap.Tables == nil {
				snap.Tables = make(map[string]json.RawMessage)
			}
			snap.Tables["agent_apps"] = data
		}
	}
	// Serialize system_config KV into Tables["system_config"] (P0-3)
	if len(systemConfig) > 0 {
		if data, err := json.Marshal(systemConfig); err == nil {
			if snap.Tables == nil {
				snap.Tables = make(map[string]json.RawMessage)
			}
			snap.Tables["system_config"] = data
		}
	}
	// Serialize MITM domains into Tables["mitm_domains"] (P0-3/P0-4).
	// When the provider returned the table, persist it even if empty so the
	// consumer can distinguish "explicit empty (delete all)" from "absent".
	if mitmFetched {
		if mitmDomains == nil {
			mitmDomains = []MITMDomainRow{}
		}
		if data, err := json.Marshal(mitmDomains); err == nil {
			if snap.Tables == nil {
				snap.Tables = make(map[string]json.RawMessage)
			}
			snap.Tables["mitm_domains"] = data
		}
	}
	// Fetch stage succeeded: we have a validated snapshot in memory.
	persistOK := true
	var persistErr error
	if s.stateDir != "" {
		if err := WriteSnapshot(s.stateDir, snap); err != nil {
			persistOK = false
			persistErr = err
		}
	}
	// Apply stage: invoke the host consumer, then record. onUpdate has no
	// error return, so LastApplyOK reflects "the consumer ran"; a consumer that
	// needs to veto success must do so via its own gate (e.g. consistency).
	applyOK := s.onUpdate != nil
	if s.onUpdate != nil {
		s.onUpdate(snap)
	}
	s.mu.Lock()
	s.snap.Store(snap)
	s.status.LastSyncTime = time.Now()
	s.status.LastFetchOK = true
	s.status.LastPersistOK = persistOK
	s.status.LastApplyOK = applyOK
	// A failed persist must not be reported as a clean sync (P2-2). The
	// in-memory snapshot still advances (fail-open) so runtime behaviour keeps
	// using the latest data, but the error stays visible.
	s.status.LastSyncOK = persistOK
	if persistOK {
		s.status.LastError = ""
		s.status.SyncCount++
		s.failCount = 0
	} else {
		s.status.LastError = "snapshot write: " + persistErr.Error()
		s.status.ErrorCount++
		s.failCount++
	}
	s.mu.Unlock()
	return nil
}
