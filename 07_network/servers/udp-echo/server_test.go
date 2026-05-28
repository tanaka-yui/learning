package main

import (
	"net"
	"testing"
	"time"
)

func TestServerEchoesDatagram(t *testing.T) {
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := NewServer(pc)
	go srv.Serve()
	t.Cleanup(func() { _ = pc.Close() })

	client, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })

	want := []byte("hello world")
	if _, err := client.WriteTo(want, pc.LocalAddr()); err != nil {
		t.Fatal(err)
	}

	got := make([]byte, 1024)
	_ = client.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, _, err := client.ReadFrom(got)
	if err != nil {
		t.Fatal(err)
	}
	if string(got[:n]) != string(want) {
		t.Fatalf("got %q want %q", got[:n], want)
	}
}
