package main

import (
	"bytes"
	"encoding/binary"
	"net"
	"testing"
	"time"
)

func TestServerAnswersAQuery(t *testing.T) {
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := NewServer(pc, map[string]net.IP{"example.local": net.ParseIP("10.0.0.1")})
	go srv.Serve()
	t.Cleanup(func() { _ = pc.Close() })

	// Build a query for example.local A IN with TXID 0xABCD.
	var q bytes.Buffer
	hdr := Header{ID: 0xABCD, Flags: 0x0100, QDCount: 1}
	q.Write(hdr.Encode())
	q.Write(EncodeQNAME("example.local"))
	typeClass := make([]byte, 4)
	binary.BigEndian.PutUint16(typeClass[0:2], 1)
	binary.BigEndian.PutUint16(typeClass[2:4], 1)
	q.Write(typeClass)

	client, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	if _, err := client.WriteTo(q.Bytes(), pc.LocalAddr()); err != nil {
		t.Fatal(err)
	}

	resp := make([]byte, 512)
	_ = client.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, _, err := client.ReadFrom(resp)
	if err != nil {
		t.Fatal(err)
	}
	if n < 12 {
		t.Fatalf("short response: %d bytes", n)
	}
	got, _ := ParseHeader(resp[:12])
	if got.ID != 0xABCD {
		t.Fatalf("TXID mismatch: %x", got.ID)
	}
	if got.ANCount != 1 {
		t.Fatalf("ANCount %d want 1", got.ANCount)
	}
	if !bytes.Contains(resp[:n], []byte{10, 0, 0, 1}) {
		t.Fatalf("answer does not contain 10.0.0.1: %x", resp[:n])
	}
}
