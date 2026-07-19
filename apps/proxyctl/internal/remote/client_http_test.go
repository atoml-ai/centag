package remote

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_SetupStatusAndPACAndCA(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/proxy/setup/status":
			if r.Header.Get("Authorization") != "Bearer tok" {
				http.Error(w, "no", http.StatusUnauthorized)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"mode":"lan","allow_lan_clients":true,"advertise_host":"1.2.3.4","pac_url":"http://1.2.3.4/pac"}`))
		case "/api/v1/proxy/pac":
			_, _ = w.Write([]byte("PACBODY"))
		case "/api/v1/proxy/ca.crt":
			_, _ = w.Write([]byte("CERTPEM"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c, err := New(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.SetupStatus(); err == nil {
		t.Fatal("expected auth error")
	}
	c.Token = "tok"
	st, err := c.SetupStatus()
	if err != nil || st.AdvertiseHost != "1.2.3.4" {
		t.Fatalf("st=%+v err=%v", st, err)
	}
	pac, err := c.FetchPAC()
	if err != nil || pac != "PACBODY" {
		t.Fatalf("pac=%q err=%v", pac, err)
	}
	ca, err := c.DownloadCA()
	if err != nil || string(ca) != "CERTPEM" {
		t.Fatalf("ca=%q err=%v", ca, err)
	}
}

func TestNew_EmptyBase(t *testing.T) {
	if _, err := New("  "); err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateRemoteReady_Table(t *testing.T) {
	tests := []struct {
		name    string
		st      *SetupStatus
		pac     string
		wantErr bool
	}{
		{"nil", nil, "", true},
		{"ok", &SetupStatus{AllowLANClients: true, AdvertiseHost: "10.0.0.1", PACURL: "http://x"}, "PROXY 10.0.0.1:8081", false},
		{"empty advertise", &SetupStatus{AllowLANClients: true, AdvertiseHost: "", PACURL: "http://x"}, "PROXY 10.0.0.1:8081", true},
		{"loopback advertise", &SetupStatus{AllowLANClients: true, AdvertiseHost: "127.0.0.1", PACURL: "http://x"}, "PROXY 10.0.0.1:8081", true},
		{"empty pac url", &SetupStatus{AllowLANClients: true, AdvertiseHost: "10.0.0.1", PACURL: ""}, "PROXY 10.0.0.1:8081", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRemoteReady(tt.st, tt.pac)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tt.wantErr)
			}
		})
	}
}
