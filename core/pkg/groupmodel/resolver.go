package groupmodel

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"
)

// planProjection is shared by group_plans and user_plans (identical shapes).
const planProjection = `id, currency,
	available_backend_ids, available_models, available_pipeline_ids,
	price_type, budget_amount, budget_period, budget_start_at, budget_end_at,
	token_quota_input, token_quota_output, token_quota_period,
	token_quota_start_at, token_quota_end_at,
	rate_limit_rpm, rate_limit_tpm`

// defaultTTL bounds how long a resolved policy is cached before it is
// re-read from the database. Plan/group mutations explicitly invalidate.
const defaultTTL = 30 * time.Second

// Resolver resolves the effective policy for users. It is safe for
// concurrent use and caches per-user results for a short TTL.
type Resolver struct {
	db     *sql.DB
	driver string
	ttl    time.Duration

	mu    sync.Mutex
	cache map[int64]*cacheEntry
}

// cacheEntry holds a resolved policy and its timestamp.
type cacheEntry struct {
	at     time.Time
	policy *EffectivePolicy
}

// Option configures a Resolver.
type Option func(*Resolver)

// WithTTL overrides the default policy cache TTL (0 disables caching).
func WithTTL(d time.Duration) Option {
	return func(r *Resolver) { r.ttl = d }
}

// NewResolver creates a resolver bound to the shared database.
// driver is "sqlite" or "postgresql".
func NewResolver(db *sql.DB, driver string, opts ...Option) *Resolver {
	r := &Resolver{
		db:     db,
		driver: strings.ToLower(strings.TrimSpace(driver)),
		ttl:    defaultTTL,
		cache:  make(map[int64]*cacheEntry),
	}
	for _, o := range opts {
		o(r)
	}
	return r
}

// Invalidate drops the cached policy for a single user.
func (r *Resolver) Invalidate(userID int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.cache, userID)
}

// InvalidateGroup drops cached policies of every user inheriting the group.
// Called when a group's plan / quota / overrides change.
func (r *Resolver) InvalidateGroup(groupID string) {
	if groupID == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for uid, e := range r.cache {
		if e.policy != nil && e.policy.GroupID == groupID {
			delete(r.cache, uid)
		}
	}
}

// InvalidateAll clears the whole policy cache.
func (r *Resolver) InvalidateAll() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cache = make(map[int64]*cacheEntry)
}

// Resolve returns the effective policy for the user. A missing user or a user
// without an active plan resolves to the unlimited global default. Database
// failures are surfaced to the caller, which is expected to fail open.
func (r *Resolver) Resolve(ctx context.Context, userID int64) (*EffectivePolicy, error) {
	if r.db == nil {
		return &EffectivePolicy{}, nil
	}

	r.mu.Lock()
	if r.ttl > 0 {
		if e, ok := r.cache[userID]; ok && time.Since(e.at) < r.ttl {
			r.mu.Unlock()
			return e.policy, nil
		}
	}
	r.mu.Unlock()

	p, err := r.resolve(ctx, userID)
	if err != nil {
		return nil, err
	}

	r.mu.Lock()
	r.cache[userID] = &cacheEntry{at: time.Now(), policy: p}
	r.mu.Unlock()
	return p, nil
}

// ResolveOverride returns the pricing override that applies to a user for the
// given backend/model/price_type, honoring the user's policy mode (group
// overrides when inheriting a group, user overrides otherwise). Returns nil
// when no override applies.
func (r *Resolver) ResolveOverride(ctx context.Context, userID int64, backendID, model, priceType string) (*PricingOverride, error) {
	ep, err := r.Resolve(ctx, userID)
	if err != nil {
		return nil, err
	}
	if ep.IsGroup() {
		return r.loadGroupOverride(ctx, ep.GroupID, backendID, model, priceType)
	}
	return r.loadUserOverride(ctx, userID, backendID, model, priceType)
}

func (r *Resolver) resolve(ctx context.Context, userID int64) (*EffectivePolicy, error) {
	u, err := r.loadUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return &EffectivePolicy{}, nil
	}

	mode := strings.TrimSpace(u.policyMode)
	if mode == "" {
		mode = PolicyModeGroup // schema default
	}
	ep := &EffectivePolicy{Mode: mode, GroupID: u.groupID}

	if mode == PolicyModeGroup && u.groupID != "" {
		plan, err := r.loadGroupPlan(ctx, u.groupID)
		if err != nil {
			return nil, err
		}
		if plan != nil {
			applyPlan(ep, plan)
			quota, err := r.loadGroupQuota(ctx, u.groupID)
			if err == nil && quota != nil {
				applyGroupQuota(ep, quota)
			}
		}
		return ep, nil
	}

	// custom mode: user plan on top of the user-table limits.
	plan, err := r.loadUserPlan(ctx, userID)
	if err != nil {
		return nil, err
	}
	if plan != nil {
		applyPlan(ep, plan)
	}
	ep.DailyTokenLimit = u.dailyTokenLimit
	ep.MonthlyTokenLimit = u.monthlyTokenLimit
	return ep, nil
}

// userRow is the minimal users projection the resolver needs.
type userRow struct {
	tenantID         sql.NullString
	groupID          string
	policyMode       string
	dailyTokenLimit  int64
	monthlyTokenLimit int64
}

func (r *Resolver) loadUser(ctx context.Context, userID int64) (*userRow, error) {
	query := `SELECT id, tenant_id, group_id, policy_mode, daily_token_limit, monthly_token_limit
		FROM users WHERE id = $1`
	u := &userRow{}
	var gid, mode sql.NullString
	err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&userID, &u.tenantID, &gid, &mode, &u.dailyTokenLimit, &u.monthlyTokenLimit,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		// Pre-group-model schema: group_id / policy_mode may not exist.
		// Fall back to treating the user as custom-mode with no group.
		if isMissingColumn(err) {
			base := `SELECT id, tenant_id, daily_token_limit, monthly_token_limit FROM users WHERE id = $1`
			b := &userRow{policyMode: PolicyModeCustom}
			if err2 := r.db.QueryRowContext(ctx, base, userID).Scan(
				&userID, &b.tenantID, &b.dailyTokenLimit, &b.monthlyTokenLimit,
			); err2 != nil {
				if err2 == sql.ErrNoRows {
					return nil, nil
				}
				return nil, err2
			}
			return b, nil
		}
		return nil, err
	}
	if gid.Valid {
		u.groupID = strings.TrimSpace(gid.String)
	}
	if mode.Valid {
		u.policyMode = strings.TrimSpace(mode.String)
	}
	return u, nil
}

// planRow is a group_plans / user_plans row with the owner in Owner.
type planRow struct {
	ID                 int64
	Owner              string
	Currency           string
	AllowBackends      []string
	AllowModels        []string
	AllowPipelines     []string
	PriceType          string
	BudgetAmount       *float64
	BudgetPeriod       string
	BudgetStartAt      *time.Time
	BudgetEndAt        *time.Time
	TokenQuotaInput    *int64
	TokenQuotaOutput   *int64
	TokenQuotaPeriod   string
	TokenQuotaStartAt  *time.Time
	TokenQuotaEndAt    *time.Time
	RateLimitRPM       int
	RateLimitTPM       int
}

func (r *Resolver) loadGroupPlan(ctx context.Context, groupID string) (*planRow, error) {
	if r.db == nil {
		return nil, nil
	}
	row := r.db.QueryRowContext(ctx,
		`SELECT `+planProjection+` FROM group_plans WHERE group_id = $1 ORDER BY created_at DESC LIMIT 1`,
		groupID)
	p, err := scanPlan(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	p.Owner = groupID
	return p, nil
}

func (r *Resolver) loadUserPlan(ctx context.Context, userID int64) (*planRow, error) {
	if r.db == nil {
		return nil, nil
	}
	row := r.db.QueryRowContext(ctx,
		`SELECT `+planProjection+` FROM user_plans WHERE user_id = $1 ORDER BY created_at DESC LIMIT 1`,
		userID)
	p, err := scanPlan(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	p.Owner = fmt.Sprintf("%d", userID)
	return p, nil
}

// scanPlan scans one plan row in planProjection column order.
func scanPlan(scanner interface{ Scan(...interface{}) error }) (*planRow, error) {
	p := &planRow{}
	var (
		budgetAmount     sql.NullFloat64
		budgetStartAt    time.Time
		budgetEndAt      time.Time
		tokenInput       sql.NullInt64
		tokenOutput      sql.NullInt64
		tokenStartAt     time.Time
		tokenEndAt       time.Time
		backends, models string
		pipelines        string
	)
	err := scanner.Scan(
		&p.ID, &p.Currency,
		&backends, &models, &pipelines,
		&p.PriceType, &budgetAmount, &p.BudgetPeriod,
		&flexTime{&budgetStartAt}, &flexTime{&budgetEndAt},
		&tokenInput, &tokenOutput, &p.TokenQuotaPeriod,
		&flexTime{&tokenStartAt}, &flexTime{&tokenEndAt},
		&p.RateLimitRPM, &p.RateLimitTPM,
	)
	if err != nil {
		return nil, err
	}
	p.AllowBackends = parseAllowList(backends)
	p.AllowModels = parseAllowList(models)
	p.AllowPipelines = parseAllowList(pipelines)
	if budgetAmount.Valid {
		p.BudgetAmount = &budgetAmount.Float64
	}
	if !budgetStartAt.IsZero() {
		p.BudgetStartAt = &budgetStartAt
	}
	if !budgetEndAt.IsZero() {
		p.BudgetEndAt = &budgetEndAt
	}
	if tokenInput.Valid {
		p.TokenQuotaInput = &tokenInput.Int64
	}
	if tokenOutput.Valid {
		p.TokenQuotaOutput = &tokenOutput.Int64
	}
	if !tokenStartAt.IsZero() {
		p.TokenQuotaStartAt = &tokenStartAt
	}
	if !tokenEndAt.IsZero() {
		p.TokenQuotaEndAt = &tokenEndAt
	}
	return p, nil
}

// groupQuotaRow is the group_quotas projection used by the shared-pool gate.
type groupQuotaRow struct {
	dailyLimit          int64
	monthlyLimit        int64
	dailyRequestLimit   int64
	monthlyRequestLimit int64
	maxBackends         int
	maxAPIKeys          int
}

func (r *Resolver) loadGroupQuota(ctx context.Context, groupID string) (*groupQuotaRow, error) {
	if r.db == nil {
		return nil, nil
	}
	q := &groupQuotaRow{}
	err := r.db.QueryRowContext(ctx,
		`SELECT daily_limit, monthly_limit, daily_request_limit, monthly_request_limit,
			max_backends, max_api_keys
		FROM group_quotas WHERE group_id = $1`, groupID).
		Scan(&q.dailyLimit, &q.monthlyLimit, &q.dailyRequestLimit, &q.monthlyRequestLimit,
			&q.maxBackends, &q.maxAPIKeys)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return q, nil
}

func (r *Resolver) loadUserOverride(ctx context.Context, userID int64, backendID, model, priceType string) (*PricingOverride, error) {
	if r.db == nil {
		return nil, nil
	}
	return scanOverride(r.db.QueryRowContext(ctx, `
		SELECT backend_id, model, price_type, input_price_per_m, output_price_per_m, currency
		FROM user_pricing_overrides
		WHERE user_id = $1 AND backend_id = $2 AND model = $3 AND price_type = $4
		  AND (effective_at IS NULL OR effective_at <= $5)
		  AND (expires_at IS NULL OR expires_at > $5)
		ORDER BY id DESC
		LIMIT 1`, userID, backendID, model, priceType, time.Now()))
}

func (r *Resolver) loadGroupOverride(ctx context.Context, groupID, backendID, model, priceType string) (*PricingOverride, error) {
	if r.db == nil {
		return nil, nil
	}
	return scanOverride(r.db.QueryRowContext(ctx, `
		SELECT backend_id, model, price_type, input_price_per_m, output_price_per_m, currency
		FROM group_pricing_overrides
		WHERE group_id = $1 AND backend_id = $2 AND model = $3 AND price_type = $4
		  AND (effective_at IS NULL OR effective_at <= $5)
		  AND (expires_at IS NULL OR expires_at > $5)
		ORDER BY id DESC
		LIMIT 1`, groupID, backendID, model, priceType, time.Now()))
}

func scanOverride(scanner interface{ Scan(...interface{}) error }) (*PricingOverride, error) {
	o := &PricingOverride{}
	err := scanner.Scan(&o.BackendID, &o.Model, &o.PriceType, &o.InputPricePerM, &o.OutputPricePerM, &o.Currency)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return o, nil
}

func applyPlan(ep *EffectivePolicy, p *planRow) {
	ep.HasPlan = true
	ep.AllowBackends = p.AllowBackends
	ep.AllowModels = p.AllowModels
	ep.AllowPipelines = p.AllowPipelines
	ep.PriceType = p.PriceType
	ep.BudgetAmount = p.BudgetAmount
	ep.BudgetPeriod = p.BudgetPeriod
	ep.BudgetStartAt = p.BudgetStartAt
	ep.BudgetEndAt = p.BudgetEndAt
	ep.TokenQuotaInput = p.TokenQuotaInput
	ep.TokenQuotaOutput = p.TokenQuotaOutput
	ep.TokenQuotaPeriod = p.TokenQuotaPeriod
	ep.TokenQuotaStartAt = p.TokenQuotaStartAt
	ep.TokenQuotaEndAt = p.TokenQuotaEndAt
	ep.RateLimitRPM = p.RateLimitRPM
	ep.RateLimitTPM = p.RateLimitTPM
}

func applyGroupQuota(ep *EffectivePolicy, q *groupQuotaRow) {
	ep.GroupDailyTokenLimit = q.dailyLimit
	ep.GroupMonthlyTokenLimit = q.monthlyLimit
	ep.GroupDailyRequestLimit = q.dailyRequestLimit
	ep.GroupMonthlyRequestLimit = q.monthlyRequestLimit
	ep.GroupMaxBackends = q.maxBackends
	ep.GroupMaxAPIKeys = q.maxAPIKeys
}

// parseAllowList parses an allowlist column which may be:
//   - empty or the JSON/PG default ('[]' / '{}') → nil (all allowed)
//   - a PG array literal {a,b,c}
//   - a comma-joined string a,b,c
func parseAllowList(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" || s == "[]" || s == "{}" {
		return nil
	}
	if strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}") {
		s = s[1 : len(s)-1]
	}
	var out []string
	for _, part := range strings.Split(s, ",") {
		p := strings.TrimSpace(part)
		p = strings.Trim(p, `"`)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// isMissingColumn reports whether the error is caused by an absent column
// (SQLite: "no such column"; PostgreSQL: "column ... does not exist").
func isMissingColumn(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "no such column") ||
		strings.Contains(msg, "does not exist")
}

// flexTime scans a nullable timestamp regardless of how the driver returns it
// (time.Time on PostgreSQL, text on SQLite) and is tolerant of NULL.
type flexTime struct {
	t *time.Time
}

func (f *flexTime) Scan(v interface{}) error {
	if f == nil || f.t == nil {
		return nil
	}
	switch s := v.(type) {
	case nil:
		return nil
	case time.Time:
		*f.t = s
		return nil
	case []byte:
		return f.parse(string(s))
	case string:
		return f.parse(s)
	default:
		return fmt.Errorf("unsupported time value %T", v)
	}
}

func (f *flexTime) parse(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	// modernc stores bound time.Time as time.Time.String(), which carries a
	// trailing " m=+..." monotonic fragment that time.Parse cannot handle.
	if i := strings.Index(s, " m="); i > 0 {
		s = s[:i]
	}
	for _, layout := range []string{
		"2006-01-02 15:04:05",
		"2006-01-02 15:04:05.999999999 -0700 MST",
		"2006-01-02T15:04:05.999999999-07:00",
		time.RFC3339,
		"2006-01-02",
	} {
		if parsed, err := time.Parse(layout, s); err == nil {
			*f.t = parsed
			return nil
		}
	}
	return fmt.Errorf("unsupported time format %q", s)
}
