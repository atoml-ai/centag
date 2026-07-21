// Package abevalapi exposes A/B eval admin query APIs for Team plugins.
// Pipeline PersistABEval wiring stays in open-core server.
package abevalapi

import (
	"context"
	"time"

	"centag/core/internal/abeval"
)

type Result = abeval.Result
type Summary = abeval.Summary

// AdminService is the Team admin query surface for A/B evaluation history.
type AdminService interface {
	ListResults(ctx context.Context, from, to time.Time, limit int) ([]Result, error)
	GetSummary(ctx context.Context, from, to time.Time) (*Summary, error)
}

type adapter struct{ s *abeval.Service }

// Wrap adapts the internal abeval.Service. Nil-safe (methods return errors).
func Wrap(s *abeval.Service) AdminService {
	return &adapter{s: s}
}

func (a *adapter) ListResults(ctx context.Context, from, to time.Time, limit int) ([]Result, error) {
	if a == nil || a.s == nil {
		return nil, errUnavailable
	}
	return a.s.ListResults(ctx, from, to, limit)
}

func (a *adapter) GetSummary(ctx context.Context, from, to time.Time) (*Summary, error) {
	if a == nil || a.s == nil {
		return nil, errUnavailable
	}
	return a.s.GetSummary(ctx, from, to)
}

type unavailableError struct{}

func (unavailableError) Error() string { return "ab eval service unavailable" }

var errUnavailable = unavailableError{}
