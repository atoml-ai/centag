package configsync

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// Record is a raw record from a data source (e.g., a Feishu Bitable row).
type Record struct {
	Key    string
	Fields map[string]string
}

// SourceCaps describes optional capabilities of a Source.
type SourceCaps struct {
	SupportsWrite  bool
	SupportsDiff   bool
	SupportsDelete bool
}

// Source is a "data table source": reads logical tables into raw records.
type Source interface {
	Name() string
	Fetch(ctx context.Context, table string) ([]Record, error)
	Capabilities() SourceCaps
}

// TableSpec defines how to fetch, parse, validate, and apply a logical table.
// Type safety is enforced at registration time via closures; internally everything is any.
type TableSpec struct {
	Name        string
	Fetch       func(ctx context.Context, src Source) ([]Record, error)
	Parse       func(r Record) (any, bool)
	Validate    func(item any) error
	Apply       func(ctx context.Context, items []any) error
	Default     func() []any
	Serialize   func(items []any) (json.RawMessage, error)
	Deserialize func(data json.RawMessage) ([]any, error)
}

// tableEntry is a registered table spec.
type tableEntry struct {
	spec TableSpec
}

// Registry maps logical table names to their sync adapters.
type Registry struct {
	mu     sync.RWMutex
	tables map[string]*tableEntry
}

// NewRegistry creates an empty Registry.
func NewRegistry() *Registry {
	return &Registry{tables: make(map[string]*tableEntry)}
}

// Register adds a TableSpec to the registry.
func (r *Registry) Register(spec TableSpec) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if spec.Name == "" {
		return fmt.Errorf("table spec: name is required")
	}
	if _, exists := r.tables[spec.Name]; exists {
		return fmt.Errorf("table spec: %q already registered", spec.Name)
	}

	r.tables[spec.Name] = &tableEntry{spec: spec}
	return nil
}

// RegisterKV is a convenience for registering a key-value spec (B-tier table).
func (r *Registry) RegisterKV(spec KeyValueSpec) error {
	kvAdapter := &kvTableAdapter{spec: spec}
	return r.Register(TableSpec{
		Name:        spec.Key,
		Fetch:       kvAdapter.fetch,
		Parse:       kvAdapter.parse,
		Validate:    kvAdapter.validate,
		Apply:       kvAdapter.apply,
		Default:     kvAdapter.defaultValue,
		Serialize:   kvAdapter.serialize,
		Deserialize: kvAdapter.deserialize,
	})
}

// Names returns all registered table names.
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.tables))
	for name := range r.tables {
		names = append(names, name)
	}
	return names
}

// Get returns the table entry by name.
func (r *Registry) Get(name string) (*tableEntry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.tables[name]
	return e, ok
}

// --- KeyValueSpec (B-tier: system_config KV) ---

// KVType describes the expected Go type for a KV value.
type KVType string

const (
	KVNumber KVType = "number" // float64
	KVInt    KVType = "int"    // int64
	KVBool   KVType = "bool"   // bool
	KVString KVType = "string" // string
	KVJSON   KVType = "json"   // json.RawMessage
)

// KVScope indicates whether a KV key belongs to core or pro.
type KVScope string

const (
	KVScopeCore KVScope = "core"
	KVScopePro  KVScope = "pro"
)

// KeyValueSpec defines a single key in the system_config KV table.
type KeyValueSpec struct {
	Key     string
	Type    KVType
	Scope   KVScope
	Default func() any
}

// systemConfigTable is the canonical logical table name shared with
// Snapshot.Tables["system_config"].
const systemConfigTable = "system_config"

// ApplyResult reports how many rows were persisted vs rejected by an applier (§3.2 Step 2).
type ApplyResult struct {
	Applied int
	Failed  int
}

// kvApplier persists a batch of KV rows. It is wired by the entrypoint to the
// live SystemConfigStore (P0-3); when unset, apply fails closed.
var (
	kvApplierMu sync.RWMutex
	kvApplier   func(ctx context.Context, rows []KVRow) (ApplyResult, error)
)

// SetKVApplier wires the persistence sink for the system_config KV table.
func SetKVApplier(fn func(ctx context.Context, rows []KVRow) (ApplyResult, error)) {
	kvApplierMu.Lock()
	defer kvApplierMu.Unlock()
	kvApplier = fn
}

// currentKVApplier returns the wired sink (nil when not configured).
func currentKVApplier() func(ctx context.Context, rows []KVRow) (ApplyResult, error) {
	kvApplierMu.RLock()
	defer kvApplierMu.RUnlock()
	return kvApplier
}

// kvTableAdapter adapts a KeyValueSpec to the TableSpec interface.
type kvTableAdapter struct {
	spec KeyValueSpec
}

func (a *kvTableAdapter) fetch(ctx context.Context, src Source) ([]Record, error) {
	return src.Fetch(ctx, systemConfigTable)
}

func (a *kvTableAdapter) parse(r Record) (any, bool) {
	val, ok := r.Fields["value"]
	if !ok {
		return nil, false
	}
	key := r.Key
	if key == "" {
		key = r.Fields["key"]
	}
	if key == "" {
		return nil, false
	}
	row := KVRow{
		Key:         key,
		Value:       val,
		ValueType:   r.Fields["value_type"],
		Scope:       r.Fields["scope"],
		Description: r.Fields["description"],
		Enabled:     true,
	}
	if _, ok := r.Fields["enabled"]; ok {
		row.Enabled = parseBoolField(r.Fields["enabled"])
	}
	return row, true
}

func (a *kvTableAdapter) validate(item any) error {
	row, ok := item.(KVRow)
	if !ok {
		return fmt.Errorf("kv: unexpected item type %T", item)
	}
	if row.Key == "" {
		return fmt.Errorf("kv: empty key")
	}
	return nil
}

func (a *kvTableAdapter) apply(ctx context.Context, items []any) error {
	applier := currentKVApplier()
	if applier == nil {
		return fmt.Errorf("kvTableAdapter: KV applier 未接线（P0-3）")
	}
	rows := make([]KVRow, 0, len(items))
	for _, it := range items {
		switch v := it.(type) {
		case KVRow:
			rows = append(rows, v)
		case map[string]any:
			rows = append(rows, kvRowFromMap(v))
		default:
			return fmt.Errorf("kv: unexpected item type %T", it)
		}
	}
	if len(rows) == 0 {
		return nil
	}
	_, err := applier(ctx, rows)
	return err
}

func (a *kvTableAdapter) defaultValue() []any {
	if a.spec.Default == nil {
		return nil
	}
	return []any{KVRow{Key: a.spec.Key, Value: fmt.Sprintf("%v", a.spec.Default()), ValueType: string(a.spec.Type), Scope: string(a.spec.Scope), Enabled: true}}
}

func (a *kvTableAdapter) serialize(items []any) (json.RawMessage, error) {
	rows := make([]KVRow, 0, len(items))
	for _, it := range items {
		switch v := it.(type) {
		case KVRow:
			rows = append(rows, v)
		case map[string]any:
			rows = append(rows, kvRowFromMap(v))
		default:
			return nil, fmt.Errorf("kv: unexpected item type %T", it)
		}
	}
	return json.Marshal(rows)
}

func (a *kvTableAdapter) deserialize(data json.RawMessage) ([]any, error) {
	var rows []KVRow
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, err
	}
	items := make([]any, len(rows))
	for i := range rows {
		items[i] = rows[i]
	}
	return items, nil
}

// kvRowFromMap normalizes an untyped JSON object into a KVRow.
func kvRowFromMap(m map[string]any) KVRow {
	row := KVRow{}
	if v, ok := m["key"].(string); ok {
		row.Key = v
	}
	if v, ok := m["value"].(string); ok {
		row.Value = v
	}
	if v, ok := m["value_type"].(string); ok {
		row.ValueType = v
	}
	if v, ok := m["scope"].(string); ok {
		row.Scope = v
	}
	if v, ok := m["description"].(string); ok {
		row.Description = v
	}
	if v, ok := m["enabled"].(bool); ok {
		row.Enabled = v
	}
	return row
}

// parseBoolField coerces common truthy string forms.
func parseBoolField(s string) bool {
	switch s {
	case "1", "true", "TRUE", "True", "yes", "on":
		return true
	default:
		return false
	}
}

// RegisterSystemConfig registers the system_config KV table under the canonical
// name "system_config", matching Snapshot.Tables["system_config"]. This is the
// production entry point for the KV table adapter (P0-3).
func (r *Registry) RegisterSystemConfig() error {
	a := &kvTableAdapter{spec: KeyValueSpec{Key: systemConfigTable, Type: KVJSON, Scope: KVScopeCore}}
	return r.Register(TableSpec{
		Name:        systemConfigTable,
		Fetch:       a.fetch,
		Parse:       a.parse,
		Validate:    a.validate,
		Apply:       a.apply,
		Default:     a.defaultValue,
		Serialize:   a.serialize,
		Deserialize: a.deserialize,
	})
}

// --- Orchestrator ---

// TableError records a per-table error from Orchestrator.Build.
type TableError struct {
	Table string
	Err   error
}

func (e TableError) Error() string {
	return fmt.Sprintf("table %s: %v", e.Table, e.Err)
}

// Orchestrator drives the fetch → parse → validate → build-candidate cycle.
type Orchestrator struct {
	registry *Registry
}

// NewOrchestrator creates an Orchestrator with the given Registry.
func NewOrchestrator(reg *Registry) *Orchestrator {
	return &Orchestrator{registry: reg}
}

// Build fetches all registered tables from the source, parses and validates
// each record, and builds a candidate Snapshot. Single-table failures do not
// block other tables (recorded in returned []TableError).
func (o *Orchestrator) Build(ctx context.Context, src Source, prev *Snapshot) (*Snapshot, []TableError) {
	var errs []TableError
	tables := make(map[string]json.RawMessage)

	for _, name := range o.registry.Names() {
		entry, _ := o.registry.Get(name)

		// 1. Fetch
		records, err := entry.spec.Fetch(ctx, src)
		if err != nil {
			errs = append(errs, TableError{Table: name, Err: fmt.Errorf("fetch: %w", err)})
			// Fallback: last-good → default
			if prev != nil {
				if data, ok := prev.Tables[name]; ok {
					tables[name] = data
					continue
				}
			}
			if defaults := entry.spec.Default(); len(defaults) > 0 {
				if data, err := entry.spec.Serialize(defaults); err == nil {
					tables[name] = data
				}
			}
			continue
		}

		// 2. Parse
		var parsed []any
		for _, rec := range records {
			if v, ok := entry.spec.Parse(rec); ok {
				parsed = append(parsed, v)
			}
		}

		// 3. Validate
		for _, item := range parsed {
			if err := entry.spec.Validate(item); err != nil {
				errs = append(errs, TableError{Table: name, Err: fmt.Errorf("validate: %w", err)})
				parsed = nil // reject entire batch on validation error
				break
			}
		}

		// 4. Serialize into Snapshot.Tables
		if len(parsed) > 0 {
			if data, err := entry.spec.Serialize(parsed); err == nil {
				tables[name] = data
			}
		}
	}

	snap := &Snapshot{
		Schema:      snapshotSchema,
		GeneratedAt: time.Now(),
		Tables:      tables,
	}

	// Carry forward legacy fields from previous snapshot
	if prev != nil {
		snap.Config = prev.Config
		snap.Prices = prev.Prices
		snap.PipelineTemplates = prev.PipelineTemplates
		snap.Skills = prev.Skills
	}

	return snap, errs
}

// Apply calls each registered table's Apply function with the data from the snapshot.
func (o *Orchestrator) Apply(ctx context.Context, snap *Snapshot) []TableError {
	var errs []TableError

	for _, name := range o.registry.Names() {
		entry, _ := o.registry.Get(name)

		data, ok := snap.Tables[name]
		if !ok {
			continue
		}

		items, err := entry.spec.Deserialize(data)
		if err != nil {
			errs = append(errs, TableError{Table: name, Err: fmt.Errorf("deserialize: %w", err)})
			continue
		}

		if err := entry.spec.Apply(ctx, items); err != nil {
			errs = append(errs, TableError{Table: name, Err: fmt.Errorf("apply: %w", err)})
		}
	}

	return errs
}
