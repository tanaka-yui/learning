package main

import (
	"errors"
	"io"
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
		s.log.Info("accept", "remote", conn.RemoteAddr().String())
		go s.handle(conn)
	}
}

func (s *Server) handle(conn net.Conn) {
	defer conn.Close()
	buf := make([]byte, 4096)
	for {
		n, err := conn.Read(buf)
		if n > 0 {
			s.log.Info("read", "n", n)
			if _, werr := conn.Write(buf[:n]); werr != nil {
				s.log.Error("write_err", "err", werr.Error())
				return
			}
		}
		if err != nil {
			if !errors.Is(err, io.EOF) {
				s.log.Error("read_err", "err", err.Error())
			}
			return
		}
	}
}
