// Package billingasync exposes the async billing event service for Team plugins.
// Pricing/rules for personal remain in core/internal/billing; this facade only
// re-exports the async recorder path so centag-pro need not import internal.
package billingasync

import (
	"centag/core/internal/billing"
)

const DefaultPricingCurrency = billing.DefaultPricingCurrency

type Event = billing.Event
type EventHandler = billing.EventHandler
type Service = billing.Service
type LogHandler = billing.LogHandler
type MockHandler = billing.MockHandler

func NewService() *Service { return billing.NewService() }

func NewMockHandler() *MockHandler { return billing.NewMockHandler() }

func NewRequestEvent(userID int64, teamID, backend, model string, tokens int64) *Event {
	return billing.NewRequestEvent(userID, teamID, backend, model, tokens)
}
