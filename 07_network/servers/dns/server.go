package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"log/slog"
	"net"
)

// Response flag values per RFC 1035 §4.1.1.
// Both responses set QR=1 and AA=1 (we are authoritative for the static zone)
// and leave RD/RA at 0 since we do not perform recursion.
const (
	flagsNoError  uint16 = 0x8400 // QR=1 AA=1 RCODE=0 (NOERROR)
	flagsNXDomain uint16 = 0x8403 // QR=1 AA=1 RCODE=3 (NXDOMAIN)
)

type Server struct {
	pc   net.PacketConn
	zone map[string]net.IP
	log  *slog.Logger
}

func NewServer(pc net.PacketConn, zone map[string]net.IP) *Server {
	return &Server{pc: pc, zone: zone, log: slog.Default()}
}

func (s *Server) Serve() error {
	buf := make([]byte, 512)
	for {
		n, addr, err := s.pc.ReadFrom(buf)
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}
		resp, err := s.handle(buf[:n])
		if err != nil {
			s.log.Error("handle_err", "err", err.Error())
			continue
		}
		_, _ = s.pc.WriteTo(resp, addr)
	}
}

func (s *Server) handle(msg []byte) ([]byte, error) {
	if len(msg) < 12 {
		return nil, errors.New("short message")
	}
	h, err := ParseHeader(msg[:12])
	if err != nil {
		return nil, err
	}
	s.log.Info("L5: parsed TXID", "txid", h.ID)
	if h.QDCount != 1 {
		return nil, errors.New("only single-question queries supported")
	}
	r := bytes.NewReader(msg[12:])
	name, err := DecodeQNAME(r)
	if err != nil {
		return nil, err
	}
	s.log.Info("L6: decoded QNAME", "name", name)
	tc := make([]byte, 4)
	if _, err := io.ReadFull(r, tc); err != nil {
		return nil, err
	}
	qtype := binary.BigEndian.Uint16(tc[0:2])
	qclass := binary.BigEndian.Uint16(tc[2:4])
	if qtype != 1 || qclass != 1 {
		return nil, errors.New("only A/IN supported")
	}

	ip, ok := s.zone[name]
	if !ok {
		s.log.Info("L7: NXDOMAIN", "name", name)
		out := Header{ID: h.ID, Flags: flagsNXDomain, QDCount: 1}.Encode()
		out = append(out, EncodeQNAME(name)...)
		out = append(out, tc...)
		return out, nil
	}
	s.log.Info("L7: answering A record", "name", name, "ip", ip.String())

	out := Header{ID: h.ID, Flags: flagsNoError, QDCount: 1, ANCount: 1}.Encode()
	out = append(out, EncodeQNAME(name)...)
	out = append(out, tc...)
	out = append(out, EncodeAnswer(name, 60, ip)...)
	return out, nil
}
