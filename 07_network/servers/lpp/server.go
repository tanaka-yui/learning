package main

import (
	"encoding/binary"
	"errors"
	"log/slog"
	"net"
	"time"
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
	for {
		cmd, payload, err := ReadFrame(conn)
		if err != nil {
			return
		}
		s.log.Info("L6: decoded frame", "cmd", cmd, "payload_len", len(payload))
		respCmd, respPayload := dispatch(cmd, payload)
		s.log.Info("L7: dispatch", "in", cmd, "out", respCmd)
		if err := WriteFrame(conn, respCmd, respPayload); err != nil {
			return
		}
	}
}

func dispatch(cmd byte, payload []byte) (byte, []byte) {
	switch cmd {
	case CmdPing:
		return CmdPing, nil
	case CmdEcho:
		return CmdEcho, payload
	case CmdTime:
		out := make([]byte, 8)
		binary.BigEndian.PutUint64(out, uint64(time.Now().UnixNano()))
		return CmdTime, out
	default:
		return CmdUnknown, nil
	}
}
