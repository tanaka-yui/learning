package main

import (
	"bufio"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestServer(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := NewServer(ln)
	go srv.Serve()
	t.Cleanup(func() { _ = ln.Close() })

	cases := []struct {
		name     string
		req      string
		wantCode int
		wantBody string
	}{
		{"root", "GET / HTTP/1.1\r\nHost: x\r\n\r\n", 200, "Hello, world!\n"},
		{"not found", "GET /missing HTTP/1.1\r\nHost: x\r\n\r\n", 404, ""},
		{"method not allowed", "POST / HTTP/1.1\r\nHost: x\r\nContent-Length: 0\r\n\r\n", 405, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			conn, err := net.Dial("tcp", ln.Addr().String())
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
			if _, err := conn.Write([]byte(c.req)); err != nil {
				t.Fatal(err)
			}
			resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
			if err != nil {
				t.Fatal(err)
			}
			if resp.StatusCode != c.wantCode {
				t.Fatalf("status %d want %d", resp.StatusCode, c.wantCode)
			}
			body := make([]byte, 64)
			n, _ := resp.Body.Read(body)
			if c.wantBody != "" && !strings.HasPrefix(string(body[:n]), c.wantBody) {
				t.Fatalf("body %q want prefix %q", body[:n], c.wantBody)
			}
		})
	}
}
