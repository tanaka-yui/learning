package main

import (
	"bufio"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
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
	fmt.Fprintln(os.Stderr, "connected, waiting for messages...")
	for {
		op, payload, err := readUnmaskedFrame(c)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return
		}
		if op == 0x1 {
			fmt.Println(string(payload))
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

func readUnmaskedFrame(c net.Conn) (byte, []byte, error) {
	hdr := make([]byte, 2)
	if _, err := io.ReadFull(c, hdr); err != nil {
		return 0, nil, err
	}
	op := hdr[0] & 0x0F
	plen := uint64(hdr[1] & 0x7F)
	switch plen {
	case 126:
		ext := make([]byte, 2)
		if _, err := io.ReadFull(c, ext); err != nil {
			return 0, nil, err
		}
		plen = uint64(binary.BigEndian.Uint16(ext))
	case 127:
		ext := make([]byte, 8)
		if _, err := io.ReadFull(c, ext); err != nil {
			return 0, nil, err
		}
		plen = binary.BigEndian.Uint64(ext)
	}
	body := make([]byte, plen)
	if _, err := io.ReadFull(c, body); err != nil {
		return 0, nil, err
	}
	return op, body, nil
}
