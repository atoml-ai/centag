package tokenusage

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// CacheSavingRecord 缓存命中节省记录。
type CacheSavingRecord struct {
	UserID           int64   `json:"user_id"`
	BackendID        string  `json:"backend_id"`
	Model            string  `json:"model"`
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	TotalTokens      int     `json:"total_tokens"`
	SavedUSD         float64 `json:"saved_usd"`
	CacheLayer       string  `json:"cache_layer"`
	TenantID         string  `json:"tenant_id"`
	DeptTag          string  `json:"dept_tag"`
	RequestID        string  `json:"request_id"`
	PipelineID       string  `json:"pipeline_id"`
}

// RecordCacheSaving 写入缓存命中节省（异步调用）。
func (s *Service) RecordCacheSaving(ctx context.Context, record *CacheSavingRecord) error {
	if record == nil || record.UserID == 0 {
		return nil
	}
	layer := strings.ToUpper(strings.TrimSpace(record.CacheLayer))
	if layer == "" {
		layer = "L1"
	}
	if record.TotalTokens <= 0 {
		record.PromptTokens, record.CompletionTokens, record.TotalTokens = estimateTokensFromResponse(record.PromptTokens, record.CompletionTokens, "")
	}
	if record.SavedUSD <= 0 && record.TotalTokens > 0 {
		record.SavedUSD = EstimateCost(record.BackendID, record.Model, record.PromptTokens, record.CompletionTokens)
	}
	if record.TotalTokens <= 0 || record.SavedUSD <= 0 {
		return nil
	}

	query := s.q(`
		INSERT INTO cache_savings
		(user_id, backend_id, model, prompt_tokens, completion_tokens, total_tokens, saved_usd, cache_layer, tenant_id, dept_tag, request_id, pipeline_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`)
	_, err := s.db.ExecContext(ctx, query,
		record.UserID,
		strings.TrimSpace(record.BackendID),
		strings.TrimSpace(record.Model),
		record.PromptTokens,
		record.CompletionTokens,
		record.TotalTokens,
		record.SavedUSD,
		layer,
		nullIfEmpty(record.TenantID),
		nullIfEmpty(record.DeptTag),
		nullIfEmpty(record.RequestID),
		nullIfEmpty(record.PipelineID),
	)
	return err
}

// SumCacheSavingsUSD 汇总时间窗口内的缓存节省金额。
func (s *Service) SumCacheSavingsUSD(ctx context.Context, from, to interface{}, tenantID string) (float64, error) {
	where := []string{"created_at >= $1", "created_at <= $2"}
	args := []interface{}{from, to}
	argN := 3
	if tid := strings.TrimSpace(tenantID); tid != "" {
		where = append(where, fmt.Sprintf("tenant_id = $%d", argN))
		args = append(args, tid)
	}
	query := s.q(fmt.Sprintf(`
		SELECT COALESCE(SUM(saved_usd), 0)
		FROM cache_savings
		WHERE %s
	`, strings.Join(where, " AND ")))
	var total float64
	if err := s.db.QueryRowContext(ctx, query, args...).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

// EstimateSavedTokensFromResponse 从缓存响应 JSON 估算 token（优先 usage 字段）。
func EstimateSavedTokensFromResponse(responseJSON string, fallbackPrompt int) (prompt, completion, total int) {
	responseJSON = strings.TrimSpace(responseJSON)
	if responseJSON == "" {
		if fallbackPrompt > 0 {
			return fallbackPrompt, 0, fallbackPrompt
		}
		return 0, 0, 0
	}
	var payload struct {
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal([]byte(responseJSON), &payload); err != nil {
		return estimateTokensFromResponse(fallbackPrompt, 0, responseJSON)
	}
	prompt = payload.Usage.PromptTokens
	completion = payload.Usage.CompletionTokens
	total = payload.Usage.TotalTokens
	if total > 0 {
		if prompt == 0 && completion > 0 {
			prompt = max(0, total-completion)
		}
		if completion == 0 && prompt > 0 {
			completion = max(0, total-prompt)
		}
		return prompt, completion, total
	}
	content := ""
	if len(payload.Choices) > 0 {
		content = payload.Choices[0].Message.Content
	}
	return estimateTokensFromResponse(fallbackPrompt, 0, content)
}

func estimateTokensFromResponse(fallbackPrompt, fallbackCompletion int, text string) (prompt, completion, total int) {
	prompt = fallbackPrompt
	completion = fallbackCompletion
	if completion == 0 && strings.TrimSpace(text) != "" {
		completion = len([]rune(strings.TrimSpace(text))) / 4
		if completion < 1 {
			completion = 1
		}
	}
	if prompt == 0 && completion > 0 {
		prompt = completion
	}
	total = prompt + completion
	return prompt, completion, total
}