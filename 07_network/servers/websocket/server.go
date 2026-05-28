package main

import (
	"bufio"
	"errors"
	"log/slog"
	"net"
	"sync"
)

type Server struct {
	ln  net.Listener
	hub *Hub
	log *slog.Logger
}

func NewServer(ln net.Listener) *Server {
	h := NewHub()
	go h.Run()
	return &Server{ln: ln, hub: h, log: slog.Default()}
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
	room, err := Upgrade(conn, r)
	if err != nil {
		s.log.Error("upgrade_err", "err", err.Error())
		return
	}
	s.log.Info("L5: upgraded", "room", room)

	c := &wsConn{conn: conn, send: make(chan []byte, 16)}
	s.hub.Join(room, c)
	defer s.hub.Leave(room, c)

	// writer goroutine: drains send channel, applies backpressure.
	go func() {
		for msg := range c.send {
			_ = WriteFrame(conn, OpText, msg)
		}
	}()

	for {
		op, payload, err := ReadFrame(r)
		if err != nil {
			c.close()
			return
		}
		s.log.Info("L6: frame", "op", op, "len", len(payload))
		switch op {
		case OpText:
			s.log.Info("L7: broadcast", "room", room, "len", len(payload))
			s.hub.Broadcast(room, c, payload)
		case OpPing:
			_ = WriteFrame(conn, OpPong, payload)
		case OpClose:
			_ = WriteFrame(conn, OpClose, payload)
			c.close()
			return
		}
	}
}

type wsConn struct {
	conn net.Conn
	send chan []byte
	once sync.Once
}

func (c *wsConn) Send(b []byte) {
	select {
	case c.send <- b:
	default:
		// drop on slow consumer (backpressure)
	}
}

func (c *wsConn) close() {
	c.once.Do(func() { close(c.send) })
}

func (c *wsConn) Close() error { c.close(); return nil }
