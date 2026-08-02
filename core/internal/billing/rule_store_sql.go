package billing

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"time"

	"centag/core/pkg/billing"
)

var pgPlaceholderRe = regexp.MustCompile(`\$\d+`)

// SQLRuleStore persists pricing rules in SQLite or PostgreSQL.
type SQLRuleStore struct {
	db     *sql.DB
	driver string // "postgresql" or "sqlite"
}

// NewSQLRuleStore creates a SQL-backed rule store.
func NewSQLRuleStore(db *sql.DB, driver string) *SQLRuleStore {
	return &SQLRuleStore{db: db, driver: driver}
}

func (s *SQLRuleStore) isPostgres() bool { return s.driver == "postgresql" }

func (s *SQLRuleStore) q(query string) string {
	if s.isPostgres() {
		return query
	}
	return pgPlaceholderRe.ReplaceAllString(query, "?")
}

func (s *SQLRuleStore) ListRules(ctx context.Context) ([]*billing.PricingRule, error) {
	query := s.q(`
		SELECT id, name, backend_id, model, price_type, input_price_per_m, output_price_per_m,
		       COALESCE(currency, 'USD'), priority, enabled, created_at, updated_at
		FROM pricing_rules
		ORDER BY priority DESC, id ASC
	`)
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*billing.PricingRule
	for rows.Next() {
		r, err := scanPricingRule(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *SQLRuleStore) GetRule(ctx context.Context, id int64) (*billing.PricingRule, error) {
	query := s.q(`
		SELECT id, name, backend_id, model, price_type, input_price_per_m, output_price_per_m,
		       COALESCE(currency, 'USD'), priority, enabled, created_at, updated_at
		FROM pricing_rules WHERE id = $1
	`)
	row := s.db.QueryRowContext(ctx, query, id)
	r, err := scanPricingRule(row)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("pricing rule %d not found", id)
	}
	return r, err
}

func (s *SQLRuleStore) CreateRule(ctx context.Context, rule *billing.PricingRule) error {
	if rule == nil {
		return fmt.Errorf("rule is nil")
	}
	if rule.BackendID == "" || rule.Model == "" {
		return fmt.Errorf("backend_id and model are required")
	}
	rule.Currency = DefaultPricingCurrency
	now := time.Now().UTC()
	if s.isPostgres() {
		query := `
			INSERT INTO pricing_rules
			(name, backend_id, model, price_type, input_price_per_m, output_price_per_m, currency, priority, enabled, created_at, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
			RETURNING id
		`
		return s.db.QueryRowContext(ctx, query,
			rule.Name, rule.BackendID, rule.Model, rule.PriceType,
			rule.InputPricePerM, rule.OutputPricePerM, rule.Currency,
			rule.Priority, rule.Enabled, now, now,
		).Scan(&rule.ID)
	}
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO pricing_rules
		(name, backend_id, model, price_type, input_price_per_m, output_price_per_m, currency, priority, enabled, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?)
	`,
		rule.Name, rule.BackendID, rule.Model, rule.PriceType,
		rule.InputPricePerM, rule.OutputPricePerM, rule.Currency,
		rule.Priority, boolToInt(rule.Enabled), now.Format(time.RFC3339), now.Format(time.RFC3339),
	)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	rule.ID = id
	rule.CreatedAt = now
	rule.UpdatedAt = now
	return nil
}

func (s *SQLRuleStore) UpdateRule(ctx context.Context, id int64, rule *billing.PricingRule) error {
	if rule == nil {
		return fmt.Errorf("rule is nil")
	}
	rule.Currency = DefaultPricingCurrency
	now := time.Now().UTC()
	var enabled interface{} = rule.Enabled
	var updatedAt interface{} = now
	if !s.isPostgres() {
		enabled = boolToInt(rule.Enabled)
		updatedAt = now.Format(time.RFC3339)
	}
	query := s.q(`
		UPDATE pricing_rules SET
			name = $1, backend_id = $2, model = $3, price_type = $4,
			input_price_per_m = $5, output_price_per_m = $6, currency = $7,
			priority = $8, enabled = $9, updated_at = $10
		WHERE id = $11
	`)
	res, err := s.db.ExecContext(ctx, query,
		rule.Name, rule.BackendID, rule.Model, rule.PriceType,
		rule.InputPricePerM, rule.OutputPricePerM, rule.Currency,
		rule.Priority, enabled, updatedAt, id,
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("pricing rule %d not found", id)
	}
	return nil
}

func (s *SQLRuleStore) DeleteRule(ctx context.Context, id int64) error {
	query := s.q(`DELETE FROM pricing_rules WHERE id = $1`)
	res, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("pricing rule %d not found", id)
	}
	return nil
}

func (s *SQLRuleStore) ImportFromYAML(ctx context.Context, data []byte) error {
	file, err := ParsePricingYAML(data)
	if err != nil {
		return err
	}
	NormalizePricingFileToUSD(file)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM pricing_rules`); err != nil {
		return err
	}
	now := time.Now().UTC()
	for i := range file.Rules {
		r := file.Rules[i]
		r.Currency = DefaultPricingCurrency
		if s.isPostgres() {
			_, err = tx.ExecContext(ctx, `
				INSERT INTO pricing_rules
				(name, backend_id, model, price_type, input_price_per_m, output_price_per_m, currency, priority, enabled, created_at, updated_at)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
			`, r.Name, r.BackendID, r.Model, r.PriceType, r.InputPricePerM, r.OutputPricePerM, r.Currency, r.Priority, r.Enabled, now, now)
		} else {
			_, err = tx.ExecContext(ctx, `
				INSERT INTO pricing_rules
				(name, backend_id, model, price_type, input_price_per_m, output_price_per_m, currency, priority, enabled, created_at, updated_at)
				VALUES (?,?,?,?,?,?,?,?,?,?,?)
			`, r.Name, r.BackendID, r.Model, r.PriceType, r.InputPricePerM, r.OutputPricePerM, r.Currency, r.Priority, boolToInt(r.Enabled), now.Format(time.RFC3339), now.Format(time.RFC3339))
		}
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *SQLRuleStore) ExportToYAML(ctx context.Context) ([]byte, error) {
	rules, err := s.ListRules(ctx)
	if err != nil {
		return nil, err
	}
	file := &PricingRulesFile{
		Version:  "1.0",
		Currency: DefaultPricingCurrency,
		USDToCNY: USDToCNY(),
		Rules:    make([]billing.PricingRule, 0, len(rules)),
	}
	for _, r := range rules {
		cp := *r
		cp.Currency = DefaultPricingCurrency
		file.Rules = append(file.Rules, cp)
	}
	return MarshalPricingYAML(file)
}

func (s *SQLRuleStore) CountRules(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM pricing_rules`).Scan(&n)
	return n, err
}

// EnsureSeededFromYAML always loads FX/meta from YAML; imports rules only when the table is empty.
func EnsureSeededFromYAML(ctx context.Context, store RuleStore, yamlPath string) error {
	file, err := LoadPricingYAMLFile(yamlPath)
	if err != nil {
		return err
	}
	ApplyPricingFileMeta(file)

	n, err := store.CountRules(ctx)
	if err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	data, err := MarshalPricingYAML(file)
	if err != nil {
		return err
	}
	return store.ImportFromYAML(ctx, data)
}

// ListRulesByType 按价格类型过滤规则
func (s *SQLRuleStore) ListRulesByType(ctx context.Context, priceType billing.PriceType) ([]*billing.PricingRule, error) {
	query := s.q(`
		SELECT id, name, backend_id, model, price_type, input_price_per_m, output_price_per_m,
		       COALESCE(currency, 'USD'), priority, enabled, created_at, updated_at
		FROM pricing_rules
		WHERE price_type = $1
		ORDER BY priority DESC, id ASC
	`)
	rows, err := s.db.QueryContext(ctx, query, priceType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*billing.PricingRule
	for rows.Next() {
		r, err := scanPricingRule(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// GetRuleByModelAndType 按后端、模型和价格类型获取规则
func (s *SQLRuleStore) GetRuleByModelAndType(ctx context.Context, backendID, model string, priceType billing.PriceType) (*billing.PricingRule, error) {
	rules, err := s.ListRulesByType(ctx, priceType)
	if err != nil {
		return nil, err
	}

	var best *billing.PricingRule
	bestPriority := -1 << 30

	for _, r := range rules {
		if r == nil || !r.Enabled {
			continue
		}
		if !wildcardMatch(r.BackendID, backendID) || !wildcardMatch(r.Model, model) {
			continue
		}
		if r.Priority > bestPriority {
			best = r
			bestPriority = r.Priority
		}
	}

	if best == nil {
		return nil, fmt.Errorf("no pricing rule found for %s/%s type=%s", backendID, model, priceType)
	}

	cp := *best
	return &cp, nil
}

type scannable interface {
	Scan(dest ...any) error
}

func scanPricingRule(row scannable) (*billing.PricingRule, error) {
	var r billing.PricingRule
	var enabled any
	var createdAt, updatedAt any
	err := row.Scan(
		&r.ID, &r.Name, &r.BackendID, &r.Model, &r.PriceType,
		&r.InputPricePerM, &r.OutputPricePerM, &r.Currency,
		&r.Priority, &enabled, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}
	r.Enabled = asBool(enabled)
	r.CreatedAt = asTime(createdAt)
	r.UpdatedAt = asTime(updatedAt)
	return &r, nil
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func asBool(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case int64:
		return x != 0
	case int:
		return x != 0
	case []byte:
		return string(x) == "1" || string(x) == "true"
	case string:
		return x == "1" || x == "true"
	default:
		return false
	}
}

func asTime(v any) time.Time {
	switch x := v.(type) {
	case time.Time:
		return x
	case string:
		if t, err := time.Parse(time.RFC3339, x); err == nil {
			return t
		}
		if t, err := time.Parse("2006-01-02 15:04:05", x); err == nil {
			return t
		}
	case []byte:
		return asTime(string(x))
	}
	return time.Time{}
}
