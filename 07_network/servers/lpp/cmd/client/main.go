package main

import (
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"
)

const (
	cmdPing byte = 0x01
	cmdEcho byte = 0x02
	cmdTime byte = 0x03
)

func writeFrame(w io.Writer, cmd byte, payload []byte) error {
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

func readFrame(r io.Reader) (byte, []byte, error) {
	hdr := make([]byte, 4)
	if _, err := io.ReadFull(r, hdr); err != nil {
		return 0, nil, err
	}
	n := binary.BigEndian.Uint32(hdr)
	if n == 0 {
		return 0, nil, errors.New("empty")
	}
	body := make([]byte, n)
	if _, err := io.ReadFull(r, body); err != nil {
		return 0, nil, err
	}
	return body[0], body[1:], nil
}

func main() {
	addr := flag.String("addr", "localhost:9004", "server address")
	flag.Parse()
	if flag.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "usage: client [-addr host:port] PING|ECHO <msg>|TIME")
		os.Exit(2)
	}
	conn, err := net.Dial("tcp", *addr)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))

	var cmd byte
	var payload []byte
	switch strings.ToUpper(flag.Arg(0)) {
	case "PING":
		cmd = cmdPing
	case "ECHO":
		cmd = cmdEcho
		payload = []byte(strings.Join(flag.Args()[1:], " "))
	case "TIME":
		cmd = cmdTime
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", flag.Arg(0))
		os.Exit(2)
	}
	if err := writeFrame(conn, cmd, payload); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	rcmd, rbody, err := readFrame(conn)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	switch rcmd {
	case cmdTime:
		ts := int64(binary.BigEndian.Uint64(rbody))
		fmt.Printf("TIME %s\n", time.Unix(0, ts).Format(time.RFC3339Nano))
	default:
		fmt.Printf("0x%02X %s\n", rcmd, string(rbody))
	}
}
