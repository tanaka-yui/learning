package main

import (
	"errors"
	"log/slog"
	"net"
)

type Server struct {
	pc  net.PacketConn
	log *slog.Logger
}

func NewServer(pc net.PacketConn) *Server {
	return &Server{pc: pc, log: slog.Default()}
}

func (s *Server) Serve() error {
	buf := make([]byte, 65536)
	for {
		n, addr, err := s.pc.ReadFrom(buf)
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}
		s.log.Info("read", "n", n, "from", addr.String())
		if _, werr := s.pc.WriteTo(buf[:n], addr); werr != nil {
			s.log.Info("write_err", "err", werr.Error())
		}
	}
}
