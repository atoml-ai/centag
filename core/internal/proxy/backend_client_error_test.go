package proxy

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"centag/core/pkg/backend"

	"github.com/gin-gonic/gin"
)

func TestWriteClassifiedBackendError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("maps no usable backend", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		ok := writeClassifiedBackendError(c, backend.NewNoUsableBackendError(errors.New("empty")))
		if !ok {
			t.Fatal("expected classified")
		}
		if w.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d, want 503", w.Code)
		}
		if got := w.Header().Get("X-ProxyClaw-Error-Code"); got != backend.ErrorCodeNoBackendConfigured {
			t.Fatalf("header code = %q", got)
		}
		var body map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		errObj, _ := body["error"].(map[string]interface{})
		if errObj == nil {
			t.Fatalf("body = %s", w.Body.String())
		}
		if errObj["code"] != backend.ErrorCodeNoBackendConfigured {
			t.Fatalf("code = %v", errObj["code"])
		}
		msg, _ := errObj["message"].(string)
		if msg == "" || msg != backend.ClientHintNoBackendConfigured {
			t.Fatalf("message = %q", msg)
		}
	})

	t.Run("maps missing api key", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		ok := writeClassifiedBackendError(c, backend.NewNoBackendAPIKeyError("openai"))
		if !ok {
			t.Fatal("expected classified")
		}
		if w.Header().Get("X-ProxyClaw-Error-Code") != backend.ErrorCodeNoBackendAPIKey {
			t.Fatalf("header = %q", w.Header().Get("X-ProxyClaw-Error-Code"))
		}
	})

	t.Run("unrelated error not classified", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		if writeClassifiedBackendError(c, errors.New("timeout upstream")) {
			t.Fatal("unrelated error should not classify")
		}
		if w.Code != 0 && w.Body.Len() > 0 {
			t.Fatalf("should not write body, got %s", w.Body.String())
		}
	})
}
