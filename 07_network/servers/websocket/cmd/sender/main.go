package main

import (
	"bufio"
	"crypto/rand"
	"encoding/base64"
	"flag"
	"fmt"
	"net"
	"os"
	"strings"
)

func main() {
	addr := flag.String("addr", "localhost:9005", "server address")
	room := flag.String("room", "demo", "room name")
	flag.Parse()

	c, err := net.Dial("tcp", *addr)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer c.Close()
	if err := handshake(c, *room); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "connected, type messages and press enter:")
	sc := bufio.NewScanner(os.Stdin)
	for sc.Scan() {
		if err := writeMaskedText(c, sc.Text()); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return
		}
	}
}

func handshake(c net.Conn, room string) error {
	keyBytes := make([]byte, 16)
	_, _ = rand.Read(keyBytes)
	key := base64.StdEncoding.EncodeToString(keyBytes)
	req := fmt.Sprintf(
		"GET /ws?room=%s HTTP/1.1\r\nHost: x\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n"+
			"Sec-WebSocket-Key: %s\r\nSec-WebSocket-Version: 13\r\n\r\n", room, key)
	if _, err := c.Write([]byte(req)); err != nil {
		return err
	}
	r := bufio.NewReader(c)
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return err
		}
		if strings.TrimRight(line, "\r\n") == "" {
			return nil
		}
	}
}

func writeMaskedText(c net.Conn, msg string) error {
	if len(msg) > 125 {
		return fmt.Errorf("message too long for this demo (limit 125)")
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
