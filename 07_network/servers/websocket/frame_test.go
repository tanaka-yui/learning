package main

import (
	"bytes"
	"testing"
)

func TestReadMaskedTextFrame(t *testing.T) {
	// FIN=1, opcode=1 (text), MASK=1, payload len=5, key=01020304, masked payload "hello".
	mask := []byte{0x01, 0x02, 0x03, 0x04}
	plain := []byte("hello")
	masked := make([]byte, len(plain))
	for i := range plain {
		masked[i] = plain[i] ^ mask[i%4]
	}
	raw := append([]byte{0x81, 0x85}, mask...)
	raw = append(raw, masked...)

	op, payload, err := ReadFrame(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if op != OpText {
		t.Fatalf("op %x", op)
	}
	if string(payload) != "hello" {
		t.Fatalf("payload %q", payload)
	}
}

func TestWriteUnmaskedTextFrame(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteFrame(&buf, OpText, []byte("hi")); err != nil {
		t.Fatal(err)
	}
	want := []byte{0x81, 0x02, 'h', 'i'}
	if !bytes.Equal(buf.Bytes(), want) {
		t.Fatalf("got %x want %x", buf.Bytes(), want)
	}
}
