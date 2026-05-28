package main

import (
	"encoding/binary"
	"errors"
	"io"
)

type Opcode byte

const (
	OpCont  Opcode = 0x0
	OpText  Opcode = 0x1
	OpBin   Opcode = 0x2
	OpClose Opcode = 0x8
	OpPing  Opcode = 0x9
	OpPong  Opcode = 0xA
)

// ReadFrame reads a single WebSocket frame from r.
// Server-side: incoming client frames MUST be masked (RFC 6455 §5.1).
// L6: bit-packed frame header + XOR masking.
func ReadFrame(r io.Reader) (Opcode, []byte, error) {
	hdr := make([]byte, 2)
	if _, err := io.ReadFull(r, hdr); err != nil {
		return 0, nil, err
	}
	op := Opcode(hdr[0] & 0x0F)
	masked := hdr[1]&0x80 != 0
	plen := uint64(hdr[1] & 0x7F)
	switch plen {
	case 126:
		ext := make([]byte, 2)
		if _, err := io.ReadFull(r, ext); err != nil {
			return 0, nil, err
		}
		plen = uint64(binary.BigEndian.Uint16(ext))
	case 127:
		ext := make([]byte, 8)
		if _, err := io.ReadFull(r, ext); err != nil {
			return 0, nil, err
		}
		plen = binary.BigEndian.Uint64(ext)
	}
	if !masked {
		return 0, nil, errors.New("client frame must be masked")
	}
	mask := make([]byte, 4)
	if _, err := io.ReadFull(r, mask); err != nil {
		return 0, nil, err
	}
	payload := make([]byte, plen)
	if _, err := io.ReadFull(r, payload); err != nil {
		return 0, nil, err
	}
	for i := range payload {
		payload[i] ^= mask[i%4]
	}
	return op, payload, nil
}

// WriteFrame writes a single unmasked frame (server-side).
func WriteFrame(w io.Writer, op Opcode, payload []byte) error {
	var hdr []byte
	first := byte(0x80) | byte(op) // FIN=1
	switch {
	case len(payload) <= 125:
		hdr = []byte{first, byte(len(payload))}
	case len(payload) <= 0xFFFF:
		hdr = []byte{first, 126, 0, 0}
		binary.BigEndian.PutUint16(hdr[2:], uint16(len(payload)))
	default:
		hdr = []byte{first, 127, 0, 0, 0, 0, 0, 0, 0, 0}
		binary.BigEndian.PutUint64(hdr[2:], uint64(len(payload)))
	}
	if _, err := w.Write(hdr); err != nil {
		return err
	}
	if len(payload) > 0 {
		if _, err := w.Write(payload); err != nil {
			return err
		}
	}
	return nil
}
