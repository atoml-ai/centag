package backend

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchModelsFromRemote_OpenAI(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{
				{"id": "gpt-4o"},
				{"id": "gpt-4o-mini"},
			},
		})
	}))
	defer srv.Close()

	m := &Manager{}
	models, err := m.FetchModelsFromRemote(context.Background(), &BackendConfig{
		Type:    "openai",
		BaseURL: srv.URL + "/v1",
		APIKey:  "sk-test",
		Timeout: 5,
	})
	if err != nil {
		t.Fatalf("FetchModelsFromRemote: %v", err)
	}
	if len(models) != 2 || models[0] != "gpt-4o" || models[1] != "gpt-4o-mini" {
		t.Fatalf("unexpected models: %#v", models)
	}
}

func TestFetchModelsFromRemote_Gemini(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-goog-api-key") == "" {
			http.Error(w, "missing key", http.StatusUnauthorized)
			return
		}
		if r.URL.Path != "/v1beta/models" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"models": []map[string]string{
				{"name": "models/gemini-2.0-flash"},
				{"name": "models/gemini-1.5-pro"},
			},
		})
	}))
	defer srv.Close()

	m := &Manager{}
	models, err := m.FetchModelsFromRemote(context.Background(), &BackendConfig{
		Type:    "gemini",
		BaseURL: srv.URL + "/v1beta",
		APIKey:  "AIza-test",
		Timeout: 5,
	})
	if err != nil {
		t.Fatalf("FetchModelsFromRemote gemini: %v", err)
	}
	if len(models) != 2 || models[0] != "gemini-2.0-flash" || models[1] != "gemini-1.5-pro" {
		t.Fatalf("unexpected gemini models: %#v", models)
	}
}

func TestFetchModelsFromRemote_IgnoresLocalCache(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{{"id": "remote-only"}},
		})
	}))
	defer srv.Close()

	m := &Manager{}
	cfg := &BackendConfig{
		Type:    "openai",
		BaseURL: srv.URL + "/v1",
		APIKey:  "sk-test",
		Timeout: 5,
		SupportedModels: []ModelMapping{
			{RequestedModel: "stale-local", ActualModel: "stale-local"},
		},
	}
	models, err := m.FetchModelsFromRemote(context.Background(), cfg)
	if err != nil {
		t.Fatalf("FetchModelsFromRemote: %v", err)
	}
	if len(models) != 1 || models[0] != "remote-only" {
		t.Fatalf("expected remote list, got %#v", models)
	}
}
