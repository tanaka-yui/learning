package main

import (
	"encoding/binary"
	"errors"
	"io"
	"net"
	"strings"
)

// Header is the 12-byte DNS message header.
// L6: bit-packed wire layout.
type Header struct {
	ID      uint16
	Flags   uint16
	QDCount uint16
	ANCount uint16
	NSCount uint16
	ARCount uint16
}

func ParseHeader(b []byte) (Header, error) {
	if len(b) < 12 {
		return Header{}, errors.New("short header")
	}
	return Header{
		ID:      binary.BigEndian.Uint16(b[0:2]),
		Flags:   binary.BigEndian.Uint16(b[2:4]),
		QDCount: binary.BigEndian.Uint16(b[4:6]),
		ANCount: binary.BigEndian.Uint16(b[6:8]),
		NSCount: binary.BigEndian.Uint16(b[8:10]),
		ARCount: binary.BigEndian.Uint16(b[10:12]),
	}, nil
}

func (h Header) Encode() []byte {
	b := make([]byte, 12)
	binary.BigEndian.PutUint16(b[0:2], h.ID)
	binary.BigEndian.PutUint16(b[2:4], h.Flags)
	binary.BigEndian.PutUint16(b[4:6], h.QDCount)
	binary.BigEndian.PutUint16(b[6:8], h.ANCount)
	binary.BigEndian.PutUint16(b[8:10], h.NSCount)
	binary.BigEndian.PutUint16(b[10:12], h.ARCount)
	return b
}

// DecodeQNAME reads a label-encoded name. Compressed pointers are rejected
// (this implementation supports compression on the encode side only).
// L6: label-length prefix encoding.
func DecodeQNAME(r io.ByteReader) (string, error) {
	var parts []string
	for {
		n, err := r.ReadByte()
		if err != nil {
			return "", err
		}
		if n == 0 {
			return strings.Join(parts, "."), nil
		}
		// Top two bits identify the label-type per RFC 1035 §4.1.4:
		//   0b00xxxxxx → label length (0–63)
		//   0b11xxxxxx → compressed pointer (rejected here)
		//   others     → reserved / extended (rare; treated as malformed below)
		if n&0xC0 == 0xC0 {
			return "", errors.New("compressed pointer not supported in decoder")
		}
		if n > 63 {
			return "", errors.New("invalid label length")
		}
		buf := make([]byte, n)
		for i := range buf {
			b, err := r.ReadByte()
			if err != nil {
				return "", err
			}
			buf[i] = b
		}
		parts = append(parts, string(buf))
	}
}

func EncodeQNAME(name string) []byte {
	var out []byte
	for _, label := range strings.Split(name, ".") {
		if label == "" {
			continue
		}
		out = append(out, byte(len(label)))
		out = append(out, []byte(label)...)
	}
	out = append(out, 0)
	return out
}

// EncodeAnswer builds a single A record answer.
// L6: type/class/ttl/rdlength bit packing.
func EncodeAnswer(name string, ttl uint32, ip net.IP) []byte {
	var out []byte
	out = append(out, EncodeQNAME(name)...)
	t := make([]byte, 10)
	binary.BigEndian.PutUint16(t[0:2], 1) // TYPE A
	binary.BigEndian.PutUint16(t[2:4], 1) // CLASS IN
	binary.BigEndian.PutUint32(t[4:8], ttl)
	binary.BigEndian.PutUint16(t[8:10], 4) // RDLENGTH
	out = append(out, t...)
	out = append(out, ip.To4()...)
	return out
}
