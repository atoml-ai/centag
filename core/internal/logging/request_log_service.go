package logging

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"
)

// RequestLogService provides async request logging with buffered writes
type RequestLogService struct {
	db     *sql.DB
	driver string

	// Buffer for async writes
	buffer   chan *RequestLogEntry
	bufferMu sync.Mutex
	bufferWg sync.WaitGroup

	// Batch insert settings
	batchSize    int
	flushInterval time.Duration

	// Lifecycle
	done chan struct{}
}

// RequestLogEntry represents a request log entry to be written
type RequestLogEntry struct {
	UserID       int64
	TenantID     string
	RequestID    string
	Model        string
	Backend      string
	Pipeline     string
	InputTokens  int64
	OutputTokens int64
	LatencyMs    int64
	StatusCode   int
	RequestBody  string
	ResponseBody string
	CreatedAt    time.Time
}

// NewRequestLogService creates a new RequestLogService
func NewRequestLogService(db *sql.DB, driver string) *RequestLogService {
	return &RequestLogService{
		db:            db,
		driver:        driver,
		buffer:        make(chan *RequestLogEntry, 10000),
		batchSize:     100,
		flushInterval: 5 * time.Second,
		done:          make(chan struct{}),
	}
}

// Start starts the background worker for async log writing
func (s *RequestLogService) Start(ctx context.Context) {
	s.bufferWg.Add(1)
	go s.worker(ctx)
}

// Stop gracefully stops the service and flushes remaining logs
func (s *RequestLogService) Stop() {
	close(s.done)
	s.bufferWg.Wait()
}

// LogRequest logs a request asynchronously
func (s *RequestLogService) LogRequest(ctx context.Context, entry *RequestLogEntry) {
	select {
	case s.buffer <- entry:
	default:
		// Buffer full, drop the log entry
		// In production, you might want to log a warning
	}
}

// LogRequestSync logs a request synchronously (for critical paths)
func (s *RequestLogService) LogRequestSync(ctx context.Context, entry *RequestLogEntry) error {
	return s.insertEntry(ctx, entry)
}

// QueryUserRequests queries request logs for a user
func (s *RequestLogService) QueryUserRequests(ctx context.Context, userID int64, start, end time.Time, limit, offset int) ([]*RequestLogEntry, error) {
	query := s.buildQuery("user_id = $1 AND created_at >= $2 AND created_at <= $3", limit, offset)

	rows, err := s.db.QueryContext(ctx, query, userID, start, end)
	if err != nil {
		return nil, fmt.Errorf("query user requests: %w", err)
	}
	defer rows.Close()

	return s.scanEntries(rows)
}

// QueryTeamRequests queries request logs for a team
func (s *RequestLogService) QueryTeamRequests(ctx context.Context, tenantID string, start, end time.Time, limit, offset int) ([]*RequestLogEntry, error) {
	query := s.buildQuery("tenant_id = $1 AND created_at >= $2 AND created_at <= $3", limit, offset)

	rows, err := s.db.QueryContext(ctx, query, tenantID, start, end)
	if err != nil {
		return nil, fmt.Errorf("query team requests: %w", err)
	}
	defer rows.Close()

	return s.scanEntries(rows)
}

// QueryRequestsByTimeRange queries request logs within a time range
func (s *RequestLogService) QueryRequestsByTimeRange(ctx context.Context, start, end time.Time, limit, offset int) ([]*RequestLogEntry, error) {
	query := s.buildQuery("created_at >= $1 AND created_at <= $2", limit, offset)

	rows, err := s.db.QueryContext(ctx, query, start, end)
	if err != nil {
		return nil, fmt.Errorf("query requests by time range: %w", err)
	}
	defer rows.Close()

	return s.scanEntries(rows)
}

// CountUserRequests counts request logs for a user
func (s *RequestLogService) CountUserRequests(ctx context.Context, userID int64, start, end time.Time) (int64, error) {
	query := `SELECT COUNT(*) FROM user_request_logs WHERE user_id = $1 AND created_at >= $2 AND created_at <= $3`
	var count int64
	err := s.db.QueryRowContext(ctx, query, userID, start, end).Scan(&count)
	return count, err
}

// DeleteOldLogs deletes logs older than the specified duration
func (s *RequestLogService) DeleteOldLogs(ctx context.Context, olderThan time.Duration) (int64, error) {
	query := `DELETE FROM user_request_logs WHERE created_at < ?`
	cutoff := time.Now().Add(-olderThan)

	result, err := s.db.ExecContext(ctx, query, cutoff)
	if err != nil {
		return 0, fmt.Errorf("delete old request logs: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("delete old request logs rows affected: %w", err)
	}
	return rowsAffected, nil
}

// worker is the background worker for async log writing
func (s *RequestLogService) worker(ctx context.Context) {
	defer s.bufferWg.Done()

	batch := make([]*RequestLogEntry, 0, s.batchSize)
	ticker := time.NewTicker(s.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Flush remaining logs
			s.flushBatch(ctx, batch)
			return
		case <-s.done:
			// Flush remaining logs
			s.flushBatch(context.Background(), batch)
			return
		case entry := <-s.buffer:
			batch = append(batch, entry)
			if len(batch) >= s.batchSize {
				s.flushBatch(ctx, batch)
				batch = batch[:0]
			}
		case <-ticker.C:
			if len(batch) > 0 {
				s.flushBatch(ctx, batch)
				batch = batch[:0]
			}
		}
	}
}

// flushBatch inserts a batch of entries into the database
func (s *RequestLogService) flushBatch(ctx context.Context, batch []*RequestLogEntry) {
	if len(batch) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	err := s.batchInsert(ctx, batch)
	if err != nil {
		// Log error but don't crash
		// In production, you might want to use a logger
		fmt.Printf("failed to flush request log batch: %v\n", err)
	}
}

// batchInsert inserts multiple entries in a single query
func (s *RequestLogService) batchInsert(ctx context.Context, entries []*RequestLogEntry) error {
	if len(entries) == 0 {
		return nil
	}

	query := s.buildBatchInsertQuery(len(entries))
	args := s.buildBatchInsertArgs(entries)

	_, err := s.db.ExecContext(ctx, query, args...)
	return err
}

// insertEntry inserts a single entry into the database
func (s *RequestLogService) insertEntry(ctx context.Context, entry *RequestLogEntry) error {
	query := s.buildInsertQuery()

	args := s.buildInsertArgs(entry)
	_, err := s.db.ExecContext(ctx, query, args...)
	return err
}

// buildInsertQuery builds the INSERT query for a single entry
func (s *RequestLogService) buildInsertQuery() string {
	return `INSERT INTO user_request_logs 
		(user_id, tenant_id, request_id, model, backend, pipeline, 
		 input_tokens, output_tokens, latency_ms, status_code, 
		 request_body, response_body, created_at) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
}

// buildInsertArgs builds the arguments for a single entry insert
func (s *RequestLogService) buildInsertArgs(entry *RequestLogEntry) []interface{} {
	now := time.Now()
	return []interface{}{
		entry.UserID, entry.TenantID, entry.RequestID, entry.Model, entry.Backend,
		entry.Pipeline, entry.InputTokens, entry.OutputTokens, entry.LatencyMs,
		entry.StatusCode, entry.RequestBody, entry.ResponseBody, now,
	}
}

// buildBatchInsertQuery builds the batch INSERT query
func (s *RequestLogService) buildBatchInsertQuery(count int) string {
	valuePlaceholder := `(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	placeholders := make([]string, count)
	for i := 0; i < count; i++ {
		placeholders[i] = valuePlaceholder
	}

	return fmt.Sprintf(`INSERT INTO user_request_logs 
		(user_id, tenant_id, request_id, model, backend, pipeline, 
		 input_tokens, output_tokens, latency_ms, status_code, 
		 request_body, response_body, created_at) 
		VALUES %s`, strings.Join(placeholders, ", "))
}

// buildBatchInsertArgs builds the arguments for a batch insert
func (s *RequestLogService) buildBatchInsertArgs(entries []*RequestLogEntry) []interface{} {
	args := make([]interface{}, 0, len(entries)*13)
	now := time.Now()

	for _, entry := range entries {
		args = append(args,
			entry.UserID, entry.TenantID, entry.RequestID, entry.Model, entry.Backend,
			entry.Pipeline, entry.InputTokens, entry.OutputTokens, entry.LatencyMs,
			entry.StatusCode, entry.RequestBody, entry.ResponseBody, now,
		)
	}
	return args
}

// buildQuery builds a SELECT query with conditions
func (s *RequestLogService) buildQuery(whereClause string, limit, offset int) string {
	query := fmt.Sprintf(`SELECT user_id, tenant_id, request_id, model, backend, pipeline, 
		input_tokens, output_tokens, latency_ms, status_code, 
		request_body, response_body, created_at 
		FROM user_request_logs WHERE %s 
		ORDER BY created_at DESC`, whereClause)

	if limit > 0 {
		query += fmt.Sprintf(` LIMIT %d`, limit)
	}
	if offset > 0 {
		query += fmt.Sprintf(` OFFSET %d`, offset)
	}

	return query
}

// scanEntries scans rows into RequestLogEntry slice
func (s *RequestLogService) scanEntries(rows *sql.Rows) ([]*RequestLogEntry, error) {
	var entries []*RequestLogEntry

	for rows.Next() {
		var entry RequestLogEntry
		if err := rows.Scan(
			&entry.UserID, &entry.TenantID, &entry.RequestID, &entry.Model, &entry.Backend,
			&entry.Pipeline, &entry.InputTokens, &entry.OutputTokens, &entry.LatencyMs,
			&entry.StatusCode, &entry.RequestBody, &entry.ResponseBody, &entry.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan request log entry: %w", err)
		}
		entries = append(entries, &entry)
	}

	return entries, nil
}

// CreatedAt is added for scanning purposes
type RequestLogEntryWithCreatedAt struct {
	RequestLogEntry
	CreatedAt time.Time
}
