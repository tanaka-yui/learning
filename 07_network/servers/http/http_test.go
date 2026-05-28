package main

import (
	"bufio"
	"strings"
	"testing"
)

func TestParseRequest(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		wantOK  bool
		method  string
		path    string
		host    string
		closeIt bool
	}{
		{
			name:   "simple GET",
			raw:    "GET / HTTP/1.1\r\nHost: example.com\r\n\r\n",
			wantOK: true, method: "GET", path: "/", host: "example.com",
		},
		{
			name:    "connection close",
			raw:     "GET /a HTTP/1.1\r\nHost: x\r\nConnection: close\r\n\r\n",
			wantOK:  true, method: "GET", path: "/a", host: "x", closeIt: true,
		},
		{
			name:   "malformed request line",
			raw:    "GET/HTTP/1.1\r\n\r\n",
			wantOK: false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := bufio.NewReader(strings.NewReader(c.raw))
			req, err := ParseRequest(r)
			if c.wantOK && err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if !c.wantOK {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if req.Method != c.method || req.Path != c.path || req.Host != c.host || req.Close != c.closeIt {
				t.Fatalf("got %+v", req)
			}
		})
	}
}
