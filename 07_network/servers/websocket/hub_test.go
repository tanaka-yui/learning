package main

import (
	"testing"
	"time"
)

func TestHubBroadcastsToSameRoom(t *testing.T) {
	h := NewHub()
	go h.Run()
	t.Cleanup(h.Stop)

	a := &fakeConn{out: make(chan []byte, 4)}
	b := &fakeConn{out: make(chan []byte, 4)}
	c := &fakeConn{out: make(chan []byte, 4)}

	h.Join("room1", a)
	h.Join("room1", b)
	h.Join("room2", c)

	h.Broadcast("room1", a, []byte("hello"))

	select {
	case msg := <-b.out:
		if string(msg) != "hello" {
			t.Fatalf("b got %q", msg)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("b did not receive")
	}
	select {
	case <-c.out:
		t.Fatal("c (different room) must not receive")
	case <-time.After(100 * time.Millisecond):
	}
	select {
	case <-a.out:
		t.Fatal("sender a must not receive its own message")
	case <-time.After(100 * time.Millisecond):
	}
}

type fakeConn struct{ out chan []byte }

func (f *fakeConn) Send(b []byte) { f.out <- b }
func (f *fakeConn) Close() error  { close(f.out); return nil }
