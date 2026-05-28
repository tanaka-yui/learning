package main

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"strings"
)

const wsMagicGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

// AcceptKey computes the Sec-WebSocket-Accept response per RFC 6455.
// L5: completes the WebSocket upgrade handshake.
func AcceptKey(clientKey string) string {
	h := sha1.New()
	_, _ = h.Write([]byte(clientKey + wsMagicGUID))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// Upgrade parses the HTTP upgrade request from r, writes the 101 response on conn,
// and returns the requested room name from the query string.
func Upgrade(conn net.Conn, r *bufio.Reader) (room string, err error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return "", err
	}
	parts := strings.SplitN(strings.TrimRight(line, "\r\n"), " ", 3)
	if len(parts) != 3 || parts[0] != "GET" {
		return "", errors.New("not a GET request")
	}
	path := parts[1]
	if i := strings.Index(path, "?room="); i >= 0 {
		room = path[i+len("?room="):]
	}

	var key string
	for {
		l, err := r.ReadString('\n')
		if err != nil {
			return "", err
		}
		l = strings.TrimRight(l, "\r\n")
		if l == "" {
			break
		}
		k, v, ok := strings.Cut(l, ":")
		if !ok {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(k), "Sec-WebSocket-Key") {
			key = strings.TrimSpace(v)
		}
	}
	if key == "" {
		return "", errors.New("missing Sec-WebSocket-Key")
	}
	resp := fmt.Sprintf(
		"HTTP/1.1 101 Switching Protocols\r\n"+
			"Upgrade: websocket\r\nConnection: Upgrade\r\n"+
			"Sec-WebSocket-Accept: %s\r\n\r\n", AcceptKey(key))
	if _, err := conn.Write([]byte(resp)); err != nil {
		return "", err
	}
	return room, nil
}
