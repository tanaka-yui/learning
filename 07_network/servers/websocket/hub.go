package main

import "sync"

type Sender interface {
	Send([]byte)
}

type Hub struct {
	mu    sync.Mutex
	rooms map[string]map[Sender]bool
	done  chan struct{}
}

func NewHub() *Hub {
	return &Hub{
		rooms: map[string]map[Sender]bool{},
		done:  make(chan struct{}),
	}
}

// L5: hub manages session membership per room.
func (h *Hub) Join(room string, c Sender) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.rooms[room] == nil {
		h.rooms[room] = map[Sender]bool{}
	}
	h.rooms[room][c] = true
}

func (h *Hub) Leave(room string, c Sender) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.rooms[room] != nil {
		delete(h.rooms[room], c)
	}
}

// L7: broadcast a chat message to peers in the same room.
func (h *Hub) Broadcast(room string, from Sender, msg []byte) {
	h.mu.Lock()
	peers := make([]Sender, 0, len(h.rooms[room]))
	for c := range h.rooms[room] {
		if c != from {
			peers = append(peers, c)
		}
	}
	h.mu.Unlock()
	for _, c := range peers {
		c.Send(msg)
	}
}

// Run blocks until Stop is called. Reserved for future channel-based dispatch.
func (h *Hub) Run()  { <-h.done }
func (h *Hub) Stop() { close(h.done) }
