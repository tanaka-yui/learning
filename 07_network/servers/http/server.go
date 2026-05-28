package main

import (
	"bufio"
	"errors"
	"fmt"
	"log/slog"
	"net"
)

type Server struct {
	ln  net.Listener
	log *slog.Logger
}

func NewServer(ln net.Listener) *Server {
	return &Server{ln: ln, log: slog.Default()}
}

func (s *Server) Serve() error {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}
		go s.handle(conn)
	}
}

func (s *Server) handle(conn net.Conn) {
	defer conn.Close()
	r := bufio.NewReader(conn)
	req, err := ParseRequest(r)
	if err != nil {
		s.log.Error("parse_err", "err", err.Error())
		writeResponse(conn, 400, "text/plain", []byte("bad request\n"), true)
		return
	}
	s.log.Info("L5: parsed request", "host", req.Host, "close", req.Close)
	s.log.Info("L7: route", "method", req.Method, "path", req.Path)

	switch {
	case req.Method != "GET":
		writeResponse(conn, 405, "text/plain", nil, true)
	case req.Path == "/":
		writeResponse(conn, 200, "text/plain", []byte("Hello, world!\n"), req.Close)
	default:
		writeResponse(conn, 404, "text/plain", nil, true)
	}
}

// writeResponse builds an HTTP/1.1 response by hand.
// L6: Content-Type / Content-Length headers.
func writeResponse(conn net.Conn, code int, contentType string, body []byte, closeIt bool) {
	reason := map[int]string{200: "OK", 400: "Bad Request", 404: "Not Found", 405: "Method Not Allowed"}[code]
	hdr := fmt.Sprintf("HTTP/1.1 %d %s\r\nContent-Type: %s\r\nContent-Length: %d\r\n",
		code, reason, contentType, len(body))
	if closeIt {
		hdr += "Connection: close\r\n"
	}
	hdr += "\r\n"
	_, _ = conn.Write([]byte(hdr))
	if len(body) > 0 {
		_, _ = conn.Write(body)
	}
}
