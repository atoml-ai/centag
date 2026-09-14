package main

import (
	"net"
	"testing"
)

// TestPickFreePort ensures the launcher skips an occupied port.
func TestPickFreePort(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	busy := ln.Addr().(*net.TCPAddr).Port

	got := pickFreePort(busy)
	if got == busy {
		t.Fatalf("expected a free port != %d", busy)
	}
	if got < busy {
		t.Fatalf("got %d, want >= %d", got, busy)
	}
}
