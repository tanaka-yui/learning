package main

import (
	"bufio"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

func TestServerBroadcastBetweenClients(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := NewServer(ln)
	go srv.Serve()
	t.Cleanup(func() { _ = ln.Close() })

	recv := dialAndUpgrade(t, ln.Addr().String(), "demo")
	defer recv.Close()
	send := dialAndUpgrade(t, ln.Addr().String(), "demo")
	defer send.Close()

	// give the receiver a moment to register with the hub
	time.Sleep(50 * time.Millisecond)

	if err := writeMaskedText(send, "hello"); err != nil {
		t.Fatal(err)
	}

	_ = recv.SetReadDeadline(time.Now().Add(2 * time.Second))
	hdr := make([]byte, 2)
	if _, err := io.ReadFull(recv, hdr); err != nil {
		t.Fatal(err)
	}
	if hdr[0] != 0x81 {
		t.Fatalf("opcode %x", hdr[0])
	}
	plen := int(hdr[1] & 0x7F)
	body := make([]byte, plen)
	if _, err := io.ReadFull(recv, body); err != nil {
		t.Fatal(err)
	}
	if string(body) != "hello" {
		t.Fatalf("got %q want %q", body, "hello")
	}
}

func dialAndUpgrade(t *testing.T, addr, room string) net.Conn {
	t.Helper()
	c, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	keyBytes := make([]byte, 16)
	_, _ = rand.Read(keyBytes)
	key := base64.StdEncoding.EncodeToString(keyBytes)
	req := fmt.Sprintf(
		"GET /ws?room=%s HTTP/1.1\r\nHost: x\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n"+
			"Sec-WebSocket-Key: %s\r\nSec-WebSocket-Version: 13\r\n\r\n", room, key)
	if _, err := c.Write([]byte(req)); err != nil {
		t.Fatal(err)
	}
	r := bufio.NewReader(c)
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		if strings.TrimRight(line, "\r\n") == "" {
			return c
		}
	}
}

func writeMaskedText(c net.Conn, msg string) error {
	if len(msg) > 125 {
		return fmt.Errorf("message too long for this helper (limit 125)")
	}
	mask := []byte{0xAA, 0xBB, 0xCC, 0xDD}
	masked := make([]byte, len(msg))
	for i := 0; i < len(msg); i++ {
		masked[i] = msg[i] ^ mask[i%4]
	}
	hdr := []byte{0x81, byte(0x80 | len(msg))}
	hdr = append(hdr, mask...)
	if _, err := c.Write(hdr); err != nil {
		return err
	}
	_, err := c.Write(masked)
	return err
}
