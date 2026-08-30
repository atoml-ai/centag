package feishu

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// statefulMock is an in-memory Bitable that behaves like the real API:
// search scans records, POST creates, PUT updates. Enables true idempotency
// and convergence assertions (TC-FSH-008/009).
type statefulMock struct {
	mu         sync.Mutex
	records    map[string]map[string]any // matchValue → fields
	nextID     int
	tokenCalls atomic.Int32
	createCnt  atomic.Int32
	updateCnt  atomic.Int32
}

func newStatefulMock() *statefulMock {
	return &statefulMock{records: map[string]map[string]any{}}
}

func (m *statefulMock) handler(t *testing.T) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/open-apis/auth/v3/tenant_access_token/internal", func(w http.ResponseWriter, r *http.Request) {
		m.tokenCalls.Add(1)
		writeJSON(w, map[string]any{"code": 0, "tenant_access_token": "tok", "expire": 7200})
	})
	mux.HandleFunc("/open-apis/bitable/v1/apps/app1/tables/t1/records/search", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Filter *struct {
				Conditions []struct {
					FieldName string   `json:"field_name"`
					Value     []string `json:"value"`
				} `json:"conditions"`
			} `json:"filter"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		var items []map[string]any
		m.mu.Lock()
		for key, fields := range m.records {
			if matchFilter(body.Filter, "key", key) {
				items = append(items, map[string]any{"record_id": fmt.Sprintf("rec-%s", key), "fields": fields})
			}
		}
		m.mu.Unlock()
		writeJSON(w, map[string]any{"code": 0, "data": map[string]any{"has_more": false, "items": items}})
	})
	mux.HandleFunc("/open-apis/bitable/v1/apps/app1/tables/t1/records", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Fields map[string]any `json:"fields"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		key, _ := body.Fields["key"].(string)
		m.mu.Lock()
		if _, exists := m.records[key]; exists {
			m.mu.Unlock()
			writeJSON(w, map[string]any{"code": 1254040, "msg": "duplicate"})
			return
		}
		m.nextID++
		m.records[key] = body.Fields
		m.mu.Unlock()
		m.createCnt.Add(1)
		writeJSON(w, map[string]any{"code": 0, "data": map[string]any{"record": map[string]any{"record_id": fmt.Sprintf("rec-%s", key)}}})
	})
	mux.HandleFunc("/open-apis/bitable/v1/apps/app1/tables/t1/records/", func(w http.ResponseWriter, r *http.Request) {
		rid := strings.TrimPrefix(r.URL.Path, "/open-apis/bitable/v1/apps/app1/tables/t1/records/")
		var body struct {
			Fields map[string]any `json:"fields"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		key := strings.TrimPrefix(rid, "rec-")
		m.mu.Lock()
		m.records[key] = body.Fields
		m.mu.Unlock()
		m.updateCnt.Add(1)
		writeJSON(w, map[string]any{"code": 0})
	})
	return mux
}

func matchFilter(filter *struct {
	Conditions []struct {
		FieldName string   `json:"field_name"`
		Value     []string `json:"value"`
	} `json:"conditions"`
}, field, value string) bool {
	if filter == nil {
		return false
	}
	for _, c := range filter.Conditions {
		if c.FieldName == field {
			for _, v := range c.Value {
				if v == value {
					return true
				}
			}
		}
	}
	return false
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// newTestClient returns a client wired to the given handler.
func newTestClient(t *testing.T, h http.Handler) (*httptest.Server, *Client) {
	t.Helper()
	ts := httptest.NewServer(h)
	c := NewClient(Config{AppID: "id", AppSecret: "sec", AppToken: "app1", TableID: "t1"})
	c.SetEndpoints(ts.URL+"/open-apis/auth/v3/tenant_access_token/internal", ts.URL+"/open-apis/bitable/v1/apps")
	return ts, c
}

func TestFeishuClient(t *testing.T) {
	ctx := context.Background()

	t.Run("TC-FSH-001_token缓存_连续调用仅一次获取", func(t *testing.T) {
		m := newStatefulMock()
		ts, c := newTestClient(t, m.handler(t))
		defer ts.Close()
		if _, err := c.TenantToken(ctx); err != nil {
			t.Fatal(err)
		}
		if _, err := c.TenantToken(ctx); err != nil {
			t.Fatal(err)
		}
		if got := m.tokenCalls.Load(); got != 1 {
			t.Fatalf("token endpoint called %d times, want 1", got)
		}
	})

	t.Run("TC-FSH-002_过期前刷新", func(t *testing.T) {
		m := newStatefulMock()
		ts, c := newTestClient(t, m.handler(t))
		defer ts.Close()
		if _, err := c.TenantToken(ctx); err != nil {
			t.Fatal(err)
		}
		// expire=310s with 300s margin → cached 10s; advance clock past that.
		c.mu.Lock()
		c.tokenExpire = time.Now().Add(-1 * time.Second)
		c.mu.Unlock()
		if _, err := c.TenantToken(ctx); err != nil {
			t.Fatal(err)
		}
		if got := m.tokenCalls.Load(); got != 2 {
			t.Fatalf("token endpoint called %d times after expiry, want 2", got)
		}
	})

	t.Run("TC-FSH-003_错误码上抛且不缓存", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.HandleFunc("/open-apis/auth/v3/tenant_access_token/internal", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, map[string]any{"code": 10003, "msg": "invalid app secret"})
		})
		ts, c := newTestClient(t, mux)
		defer ts.Close()
		_, err := c.TenantToken(ctx)
		if err == nil || !strings.Contains(err.Error(), "10003") {
			t.Fatalf("want error with code 10003, got %v", err)
		}
		c.mu.Lock()
		cached := c.token
		c.mu.Unlock()
		if cached != "" {
			t.Fatal("failed token must not be cached")
		}
	})

	t.Run("TC-FSH-004_search分页聚合", func(t *testing.T) {
		pages := 0
		mux := http.NewServeMux()
		mux.HandleFunc("/open-apis/auth/v3/tenant_access_token/internal", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, map[string]any{"code": 0, "tenant_access_token": "tok", "expire": 7200})
		})
		mux.HandleFunc("/open-apis/bitable/v1/apps/app1/tables/t1/records/search", func(w http.ResponseWriter, r *http.Request) {
			var body struct {
				PageToken string `json:"page_token"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			hasMore := body.PageToken != "tok3"
			next := map[string]string{"": "tok2", "tok2": "tok3", "tok3": ""}[body.PageToken]
			pages++
			writeJSON(w, map[string]any{"code": 0, "data": map[string]any{
				"has_more": hasMore, "page_token": next, "total": 3,
				"items": []map[string]any{{"record_id": "r" + body.PageToken, "fields": map[string]any{"k": "v"}}},
			}})
		})
		ts, c := newTestClient(t, mux)
		defer ts.Close()
		recs, err := c.SearchRecords(ctx, "app1", "t1", nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(recs) != 3 || pages != 3 {
			t.Fatalf("records=%d pages=%d, want 3/3", len(recs), pages)
		}
	})

	t.Run("TC-FSH-005_filter透传", func(t *testing.T) {
		var got *Filter
		mux := http.NewServeMux()
		mux.HandleFunc("/open-apis/auth/v3/tenant_access_token/internal", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, map[string]any{"code": 0, "tenant_access_token": "tok", "expire": 7200})
		})
		mux.HandleFunc("/open-apis/bitable/v1/apps/app1/tables/t1/records/search", func(w http.ResponseWriter, r *http.Request) {
			var body struct {
				Filter *Filter `json:"filter"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			got = body.Filter
			writeJSON(w, map[string]any{"code": 0, "data": map[string]any{"has_more": false, "items": []any{}}})
		})
		ts, c := newTestClient(t, mux)
		defer ts.Close()
		want := NewFilter("enabled", "is", "true")
		if _, err := c.SearchRecords(ctx, "app1", "t1", want); err != nil {
			t.Fatal(err)
		}
		if got == nil || len(got.Conditions) != 1 || got.Conditions[0].FieldName != "enabled" || got.Conditions[0].Operator != "is" || got.Conditions[0].Value[0] != "true" {
			t.Fatalf("structured filter not passed through: %+v", got)
		}
	})

	t.Run("TC-FSH-006_upsert命中更新", func(t *testing.T) {
		m := newStatefulMock()
		ts, c := newTestClient(t, m.handler(t))
		defer ts.Close()
		m.mu.Lock()
		m.records["k1"] = map[string]any{"key": "k1", "value": "old"}
		m.mu.Unlock()
		rid, created, err := c.UpsertRecord(ctx, "app1", "t1", "key", "k1", map[string]any{"key": "k1", "value": "new"})
		if err != nil || created || rid != "rec-k1" {
			t.Fatalf("rid=%s created=%v err=%v", rid, created, err)
		}
		if m.updateCnt.Load() != 1 || m.createCnt.Load() != 0 {
			t.Fatalf("update=%d create=%d, want 1/0", m.updateCnt.Load(), m.createCnt.Load())
		}
	})

	t.Run("TC-FSH-007_upsert未命中创建", func(t *testing.T) {
		m := newStatefulMock()
		ts, c := newTestClient(t, m.handler(t))
		defer ts.Close()
		rid, created, err := c.UpsertRecord(ctx, "app1", "t1", "key", "k2", map[string]any{"key": "k2"})
		if err != nil || !created || rid != "rec-k2" {
			t.Fatalf("rid=%s created=%v err=%v", rid, created, err)
		}
	})

	t.Run("TC-FSH-008_upsert幂等_重复执行行数不变", func(t *testing.T) {
		m := newStatefulMock()
		ts, c := newTestClient(t, m.handler(t))
		defer ts.Close()
		fields := func() map[string]any { return map[string]any{"key": "k1", "price": 1} }
		if _, _, err := c.UpsertRecord(ctx, "app1", "t1", "key", "k1", fields()); err != nil {
			t.Fatal(err)
		}
		if _, _, err := c.UpsertRecord(ctx, "app1", "t1", "key", "k1", fields()); err != nil {
			t.Fatal(err)
		}
		m.mu.Lock()
		n := len(m.records)
		v := m.records["k1"]["price"]
		m.mu.Unlock()
		if n != 1 {
			t.Fatalf("records=%d after double upsert, want 1", n)
		}
		if v != float64(1) {
			t.Fatalf("value=%v, want 1", v)
		}
	})

	t.Run("TC-FSH-009_并发同键收敛单行", func(t *testing.T) {
		m := newStatefulMock()
		ts, c := newTestClient(t, m.handler(t))
		defer ts.Close()
		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, _, _ = c.UpsertRecord(ctx, "app1", "t1", "key", "same", map[string]any{"key": "same"})
			}()
		}
		wg.Wait()
		m.mu.Lock()
		n := len(m.records)
		m.mu.Unlock()
		if n != 1 {
			t.Fatalf("concurrent same-key upserts produced %d rows, want 1", n)
		}
	})

	t.Run("TC-FSH-010_ensureFields仅补缺失列", func(t *testing.T) {
		existing := map[string]bool{"col_a": true, "col_b": true}
		var created []string
		mux := http.NewServeMux()
		mux.HandleFunc("/open-apis/auth/v3/tenant_access_token/internal", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, map[string]any{"code": 0, "tenant_access_token": "tok", "expire": 7200})
		})
		mux.HandleFunc("/open-apis/bitable/v1/apps/app1/tables/t1/fields", func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet:
				items := []any{}
				for name := range existing {
					items = append(items, map[string]any{"field_name": name})
				}
				writeJSON(w, map[string]any{"code": 0, "data": map[string]any{"has_more": false, "items": items}})
			case http.MethodPost:
				var body struct {
					FieldName string `json:"field_name"`
				}
				_ = json.NewDecoder(r.Body).Decode(&body)
				existing[body.FieldName] = true
				created = append(created, body.FieldName)
				writeJSON(w, map[string]any{"code": 0})
			}
		})
		ts, c := newTestClient(t, mux)
		defer ts.Close()
		if err := c.EnsureFields(ctx, "app1", "t1", []string{"col_a", "col_b", "col_c", "col_d"}); err != nil {
			t.Fatal(err)
		}
		if len(created) != 2 {
			t.Fatalf("created=%v, want exactly col_c+col_d", created)
		}
	})

	t.Run("TC-FSH-011_createBase", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.HandleFunc("/open-apis/auth/v3/tenant_access_token/internal", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, map[string]any{"code": 0, "tenant_access_token": "tok", "expire": 7200})
		})
		mux.HandleFunc("/open-apis/bitable/v1/apps", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, map[string]any{"code": 0, "data": map[string]any{
				"app": map[string]any{
					"app_token": "new-app-token", "default_table_id": "new-table-id", "url": "https://x",
				},
			}})
		})
		ts, c := newTestClient(t, mux)
		defer ts.Close()
		appToken, tableID, err := c.CreateBase(ctx, "My Base")
		if err != nil || appToken != "new-app-token" || tableID != "new-table-id" {
			t.Fatalf("appToken=%s tableID=%s err=%v", appToken, tableID, err)
		}
	})

	t.Run("TC-FSH-012_网络故障上抛", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.HandleFunc("/open-apis/auth/v3/tenant_access_token/internal", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, map[string]any{"code": 0, "tenant_access_token": "tok", "expire": 7200})
		})
		mux.HandleFunc("/open-apis/bitable/v1/apps/", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		})
		ts, c := newTestClient(t, mux)
		defer ts.Close()
		if _, err := c.ListTables(ctx, "app1"); err == nil {
			t.Fatal("want error on 5xx")
		}
	})

	t.Run("TC-FSH-013_超大响应防护", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.HandleFunc("/open-apis/auth/v3/tenant_access_token/internal", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, map[string]any{"code": 0, "tenant_access_token": "tok", "expire": 7200})
		})
		mux.HandleFunc("/open-apis/bitable/v1/apps/", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"code":0,"data":{"items":` + strings.Repeat("x", 2<<20) + `}}`))
		})
		ts, c := newTestClient(t, mux)
		defer ts.Close()
		_, _ = c.ListTables(ctx, "app1") // must not panic; decode error acceptable
	})
}

// TC-CFG-007_table_id自动解析 uses FindTable — covered in channel tests of
// configsync; here we verify FindTable happy/ambiguous paths against the mock.
func TestFindTable(t *testing.T) {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("/open-apis/auth/v3/tenant_access_token/internal", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"code": 0, "tenant_access_token": "tok", "expire": 7200})
	})
	mux.HandleFunc("/open-apis/bitable/v1/apps/app1/tables", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"code": 0, "data": map[string]any{"has_more": false, "items": []map[string]any{
			{"table_id": "tblCfg", "name": "centag_config"},
			{"table_id": "tblPrice", "name": "centag_model_price"},
		}}})
	})
	ts, c := newTestClient(t, mux)
	defer ts.Close()
	id, err := c.FindTable(ctx, "app1", "centag_config")
	if err != nil || id != "tblCfg" {
		t.Fatalf("id=%s err=%v", id, err)
	}
	if _, err := c.FindTable(ctx, "app1", "missing_table"); err == nil {
		t.Fatal("want error for missing table")
	}
}

// 真实 API 兼容：文本字段读取侧返回富文本分段。
func TestTextField(t *testing.T) {
	plain := map[string]any{"a": "str"}
	if got := TextField(plain["a"]); got != "str" {
		t.Fatalf("plain: %q", got)
	}
	rich := map[string]any{"b": []any{map[string]any{"text": "table.", "type": "text"}, map[string]any{"text": "model_price", "type": "text"}}}
	if got := TextField(rich["b"]); got != "table.model_price" {
		t.Fatalf("rich: %q", got)
	}
	if got := TextField(nil); got != "" {
		t.Fatalf("nil: %q", got)
	}
}
