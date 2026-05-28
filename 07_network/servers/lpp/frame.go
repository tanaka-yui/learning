package main

import (
	"encoding/binary"
	"errors"
	"io"
)

const (
	CmdPing    byte = 0x01
	CmdEcho    byte = 0x02
	CmdTime    byte = 0x03
	CmdUnknown byte = 0xFF
)

const maxFrame = 1 << 20 // 1 MiB safety cap

// WriteFrame writes a length-prefixed frame: Len(4) | Cmd(1) | Payload.
// L6: binary length + cmd byte encoding.
func WriteFrame(w io.Writer, cmd byte, payload []byte) error {
	body := make([]byte, 1+len(payload))
	body[0] = cmd
	copy(body[1:], payload)
	hdr := make([]byte, 4)
	binary.BigEndian.PutUint32(hdr, uint32(len(body)))
	if _, err := w.Write(hdr); err != nil {
		return err
	}
	_, err := w.Write(body)
	return err
}

func ReadFrame(r io.Reader) (byte, []byte, error) {
	hdr := make([]byte, 4)
	if _, err := io.ReadFull(r, hdr); err != nil {
		return 0, nil, err
	}
	n := binary.BigEndian.Uint32(hdr)
	if n == 0 {
		return 0, nil, errors.New("empty frame")
	}
	if n > maxFrame {
		return 0, nil, errors.New("frame too large")
	}
	body := make([]byte, n)
	if _, err := io.ReadFull(r, body); err != nil {
		return 0, nil, err
	}
	return body[0], body[1:], nil
}
