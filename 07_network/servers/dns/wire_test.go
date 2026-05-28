package main

import (
	"bytes"
	"testing"
)

func TestDecodeQNAME(t *testing.T) {
	cases := []struct {
		name    string
		raw     []byte
		want    string
		wantErr bool
	}{
		{"single label", []byte{3, 'w', 'w', 'w', 0}, "www", false},
		{"two labels", []byte{7, 'e', 'x', 'a', 'm', 'p', 'l', 'e', 3, 'c', 'o', 'm', 0}, "example.com", false},
		{"pointer (unsupported)", []byte{0xC0, 0x00}, "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := bytes.NewReader(c.raw)
			name, err := DecodeQNAME(r)
			if c.wantErr {
				if err == nil {
					t.Fatal("want err")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if name != c.want {
				t.Fatalf("got %q want %q", name, c.want)
			}
		})
	}
}

func TestEncodeQNAMERoundtrip(t *testing.T) {
	encoded := EncodeQNAME("example.com")
	got, err := DecodeQNAME(bytes.NewReader(encoded))
	if err != nil {
		t.Fatal(err)
	}
	if got != "example.com" {
		t.Fatalf("got %q", got)
	}
}

func TestParseHeader(t *testing.T) {
	raw := []byte{
		0x12, 0x34, // ID
		0x01, 0x00, // flags: RD
		0x00, 0x01, // QDCOUNT
		0x00, 0x00, // ANCOUNT
		0x00, 0x00, // NSCOUNT
		0x00, 0x00, // ARCOUNT
	}
	h, err := ParseHeader(raw)
	if err != nil {
		t.Fatal(err)
	}
	if h.ID != 0x1234 || h.QDCount != 1 {
		t.Fatalf("got %+v", h)
	}
}
