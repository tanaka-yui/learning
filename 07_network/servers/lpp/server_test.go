package main

import (
	"bytes"
	"encoding/binary"
	"net"
	"testing"
	"time"
)

func TestServerCommands(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := NewServer(ln)
	go srv.Serve()
	t.Cleanup(func() { _ = ln.Close() })

	cases := []struct {
		name     string
		cmd      byte
		payload  []byte
		wantCmd  byte
		wantBody []byte
		bodyLen  int
	}{
		{"ping", CmdPing, nil, CmdPing, nil, 0},
		{"echo", CmdEcho, []byte("hi"), CmdEcho, []byte("hi"), 2},
		{"time returns 8 bytes", CmdTime, nil, CmdTime, nil, 8},
		{"unknown", 0xAA, nil, CmdUnknown, nil, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			conn, err := net.Dial("tcp", ln.Addr().String())
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
			if err := WriteFrame(conn, c.cmd, c.payload); err != nil {
				t.Fatal(err)
			}
			cmd, body, err := ReadFrame(conn)
			if err != nil {
				t.Fatal(err)
			}
			if cmd != c.wantCmd {
				t.Fatalf("cmd %x want %x", cmd, c.wantCmd)
			}
			if c.wantBody != nil && !bytes.Equal(body, c.wantBody) {
				t.Fatalf("body %x want %x", body, c.wantBody)
			}
			if c.bodyLen > 0 && len(body) != c.bodyLen {
				t.Fatalf("body len %d want %d", len(body), c.bodyLen)
			}
			if c.cmd == CmdTime {
				ts := int64(binary.BigEndian.Uint64(body))
				if ts <= 0 {
					t.Fatalf("invalid timestamp %d", ts)
				}
			}
		})
	}
}
