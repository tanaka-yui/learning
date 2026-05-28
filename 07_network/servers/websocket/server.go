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
	conn   net.Conn
	send   chan []byte
	mu     sync.Mutex
	closed bool
}

// Send enqueues b for the writer goroutine.
// Drops b on backpressure (channel full) and silently no-ops after close,
// so a Broadcast that races Leave never panics on a closed channel.
func (c *wsConn) Send(b []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	select {
	case c.send <- b:
	default:
	}
}

func (c *wsConn) close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	c.closed = true
	close(c.send)
}

func (c *wsConn) Close() error { c.close(); return nil }
