package systemupdateapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWrap_NilIsNop(t *testing.T) {
	h := Wrap(nil)
	if h == nil {
		t.Fatal("Wrap(nil) must return nop handler")
	}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/system/update", nil)
	h.HandleUpdate(rr, req)
	h.HandleUpdateHistory(rr, req)
	h.HandleRollback(rr, req)
	h.HandleDelete(rr, req)
	h.HandleCheckUpdate(rr, req)
	h.HandleApplyRemote(rr, req)
}
