package remote

import "testing"

func TestParsePACProxyHostPort(t *testing.T) {
	pac := `function FindProxyForURL(url, host) { return "PROXY 192.168.1.4:8081"; }`
	got, err := ParsePACProxyHostPort(pac)
	if err != nil {
		t.Fatal(err)
	}
	if got != "192.168.1.4:8081" {
		t.Fatalf("got %q", got)
	}
	if _, err := ParsePACProxyHostPort("DIRECT"); err == nil {
		t.Fatal("expected error")
	}
}

func TestIsLoopbackMITM(t *testing.T) {
	if !IsLoopbackMITM("127.0.0.1:8081") {
		t.Fatal("expected loopback")
	}
	if IsLoopbackMITM("192.168.1.4:8081") {
		t.Fatal("expected non-loopback")
	}
}

func TestRejectLoopbackMITMForRemote(t *testing.T) {
	if RejectLoopbackMITMForRemote("http://127.0.0.1:20060", "127.0.0.1:8081") {
		t.Fatal("local API must allow loopback MITM")
	}
	if RejectLoopbackMITMForRemote("http://localhost:20060", "127.0.0.1:8081") {
		t.Fatal("localhost API must allow loopback MITM")
	}
	if !RejectLoopbackMITMForRemote("http://192.168.1.4:20060", "127.0.0.1:8081") {
		t.Fatal("remote API must reject loopback MITM")
	}
	if RejectLoopbackMITMForRemote("http://192.168.1.4:20060", "192.168.1.4:8081") {
		t.Fatal("remote API + LAN MITM should be allowed")
	}
}
