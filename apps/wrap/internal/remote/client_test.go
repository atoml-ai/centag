package remote

import "testing"

func TestValidateRemoteReady(t *testing.T) {
	st := &SetupStatus{
		AllowLANClients: true,
		AdvertiseHost:   "192.168.1.10",
		PACURL:          "http://192.168.1.10:20060/api/v1/proxy/pac",
	}
	pac := `return "PROXY 192.168.1.10:8081";`
	if err := ValidateRemoteReady(st, pac); err != nil {
		t.Fatal(err)
	}
	st.AllowLANClients = false
	if err := ValidateRemoteReady(st, pac); err == nil {
		t.Fatal("expected error when lan disabled")
	}
	st.AllowLANClients = true
	if err := ValidateRemoteReady(st, `PROXY 127.0.0.1:8081`); err == nil {
		t.Fatal("expected error for loopback PROXY")
	}
}
