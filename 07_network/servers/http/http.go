package main

import (
	"bufio"
	"errors"
	"strings"
)

type Request struct {
	Method  string
	Path    string
	Proto   string
	Host    string
	Headers map[string]string
	Close   bool
}

// ParseRequest reads a single HTTP/1.x request from r.
// L7: method/path semantics. L6: text framing (lines + CRLF). L5: Host + Connection.
func ParseRequest(r *bufio.Reader) (*Request, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return nil, err
	}
	line = strings.TrimRight(line, "\r\n")
	// SplitN with limit 3 keeps the parser tolerant of extra spaces inside Proto
	// while still treating "<2 fields" as malformed below.
	parts := strings.SplitN(line, " ", 3)
	if len(parts) != 3 {
		return nil, errors.New("malformed request line")
	}
	req := &Request{Method: parts[0], Path: parts[1], Proto: parts[2], Headers: map[string]string{}}

	for {
		l, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		l = strings.TrimRight(l, "\r\n")
		if l == "" {
			break
		}
		k, v, ok := strings.Cut(l, ":")
		if !ok {
			return nil, errors.New("malformed header")
		}
		k = strings.ToLower(strings.TrimSpace(k))
		v = strings.TrimSpace(v)
		req.Headers[k] = v
		switch k {
		case "host":
			req.Host = v
		case "connection":
			if strings.EqualFold(v, "close") {
				req.Close = true
			}
		}
	}
	return req, nil
}
