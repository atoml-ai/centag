package mitm

import (
	"bufio"
	"context"
	"io"
	"net"
	"os"
	"strings"
	"testing"

	"centag/core/pkg/logger"

	"go.uber.org/zap"
)

func TestMain(m *testing.M) {
	if logger.Logger == nil {
		logger.Logger = zap.NewNop()
	}
	if logger.Sugar == nil {
		logger.Sugar = zap.NewNop().Sugar()
	}
	os.Exit(m.Run())
}

func TestNormalizeProxyURL(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"127.0.0.1:12000", "http://127.0.0.1:12000"},
		{"http://127.0.0.1:12000", "http://127.0.0.1:12000"},
		{"socks5://127.0.0.1:1080", "socks5://127.0.0.1:1080"},
		{"http=127.0.0.1:12000;https=127.0.0.1:12001", "http://127.0.0.1:12001"},
		{"", ""},
		{"ftp://127.0.0.1:21", ""},
	}
	for _, tc := range cases {
		got := normalizeProxyURL(tc.in)
		if tc.want == "" {
			if got != nil {
				t.Errorf("normalizeProxyURL(%q) = %v, want nil", tc.in, got)
			}
			continue
		}
		if got == nil || got.String() != tc.want {
			t.Errorf("normalizeProxyURL(%q) = %v, want %s", tc.in, got, tc.want)
		}
	}
}

func TestUpstreamProxyForModes(t *testing.T) {
	self := []string{"127.0.0.1:8081", "127.0.0.1:20060"}

	direct := newEgressUpstream("direct", "", nil, self)
	if u := direct.proxyFor("example.com:443"); u != nil {
		t.Errorf("direct mode should not proxy, got %v", u)
	}

	manual := newEgressUpstream("manual", "http://127.0.0.1:12000", nil, self)
	if u := manual.proxyFor("chatgpt.com:443"); u == nil || u.String() != "http://127.0.0.1:12000" {
		t.Errorf("manual mode proxyFor = %v, want http://127.0.0.1:12000", u)
	}

	selfManual := newEgressUpstream("manual", "http://127.0.0.1:8081", nil, self)
	if u := selfManual.proxyFor("chatgpt.com:443"); u != nil {
		t.Errorf("manual self URL must be ignored (loop guard), got %v", u)
	}

	// Loopback and self targets always bypass, even in manual mode.
	if u := manual.proxyFor("127.0.0.1:9999"); u != nil {
		t.Errorf("loopback target must bypass upstream, got %v", u)
	}
	if u := manual.proxyFor("127.0.0.1:20060"); u != nil {
		t.Errorf("self target must bypass upstream, got %v", u)
	}
}

func TestDialHTTPConnectUpstream(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	connectLine := make(chan string, 1)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		br := bufio.NewReader(c)
		line, _ := br.ReadString('\n')
		connectLine <- strings.TrimSpace(line)
		for {
			h, err := br.ReadString('\n')
			if err != nil || h == "\r\n" || h == "\n" {
				break
			}
		}
		_, _ = c.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
		_, _ = io.Copy(c, br)
	}()

	e := newEgressUpstream("manual", ln.Addr().String(), nil, nil)
	conn, err := e.dialContext(context.Background(), "tcp", "example.com:443")
	if err != nil {
		t.Fatalf("dialContext: %v", err)
	}
	defer conn.Close()

	if got := <-connectLine; got != "CONNECT example.com:443 HTTP/1.1" {
		t.Fatalf("CONNECT line = %q", got)
	}

	if _, err := conn.Write([]byte("ping")); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 4)
	if _, err := io.ReadFull(conn, buf); err != nil {
		t.Fatalf("read echo: %v", err)
	}
	if string(buf) != "ping" {
		t.Fatalf("echo = %q, want ping", buf)
	}
}

func TestUpstreamBypassNoProxy(t *testing.T) {
	e := newEgressUpstream("direct", "", []string{"*.internal", "10.0.0.0/8", "skip.test:8443"}, nil)
	if !e.bypass("api.internal:443") {
		t.Error("wildcard no_proxy suffix should match")
	}
	if !e.bypass("10.1.2.3:443") {
		t.Error("CIDR no_proxy should match")
	}
	if !e.bypass("skip.test:8443") {
		t.Error("host:port no_proxy should match")
	}
	if e.bypass("skip.test:9443") {
		t.Error("no_proxy with port must not match a different port")
	}
	if e.bypass("example.com:443") {
		t.Error("unlisted host must not bypass")
	}
}
