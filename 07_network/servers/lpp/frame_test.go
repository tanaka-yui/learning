package main

import (
	"bytes"
	"io"
	"testing"
)

func TestEncodeDecodeFrameRoundtrip(t *testing.T) {
	cases := []struct {
		name    string
		cmd     byte
		payload []byte
	}{
		{"ping empty", CmdPing, nil},
		{"echo short", CmdEcho, []byte("hello")},
		{"time 8 bytes", CmdTime, []byte{0, 0, 0, 0, 0, 0, 0, 1}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := WriteFrame(&buf, c.cmd, c.payload); err != nil {
				t.Fatal(err)
			}
			cmd, payload, err := ReadFrame(&buf)
			if err != nil {
				t.Fatal(err)
			}
			if cmd != c.cmd {
				t.Fatalf("cmd %x want %x", cmd, c.cmd)
			}
			if !bytes.Equal(payload, c.payload) {
				t.Fatalf("payload %x want %x", payload, c.payload)
			}
		})
	}
}

func TestReadFrameShortRead(t *testing.T) {
	// Provide only the length prefix + cmd byte but missing body bytes;
	// expect io.ErrUnexpectedEOF on body.
	buf := bytes.NewReader([]byte{0, 0, 0, 5, 0x02}) // says 5 body bytes, gives 1
	_, _, err := ReadFrame(buf)
	if err != io.ErrUnexpectedEOF {
		t.Fatalf("want ErrUnexpectedEOF got %v", err)
	}
}
