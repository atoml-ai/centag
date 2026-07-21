// Package systemupdateapi exposes system-update HTTP handlers for Team plugins.
package systemupdateapi

import (
	"net/http"

	"centag/core/internal"
)

// Handler is the Team system-update HTTP surface (net/http style).
type Handler interface {
	HandleUpdate(w http.ResponseWriter, r *http.Request)
	HandleUpdateHistory(w http.ResponseWriter, r *http.Request)
	HandleRollback(w http.ResponseWriter, r *http.Request)
	HandleDelete(w http.ResponseWriter, r *http.Request)
}

// Wrap adapts the internal SystemUpdateHandler.
func Wrap(h *internal.SystemUpdateHandler) Handler {
	if h == nil {
		return nopHandler{}
	}
	return h
}

type nopHandler struct{}

func (nopHandler) HandleUpdate(http.ResponseWriter, *http.Request)        {}
func (nopHandler) HandleUpdateHistory(http.ResponseWriter, *http.Request) {}
func (nopHandler) HandleRollback(http.ResponseWriter, *http.Request)      {}
func (nopHandler) HandleDelete(http.ResponseWriter, *http.Request)        {}
