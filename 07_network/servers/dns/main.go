package main

import (
	"context"
	"flag"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	addr := flag.String("addr", ":5353", "listen address")
	flag.Parse()

	pc, err := net.ListenPacket("udp", *addr)
	if err != nil {
		slog.Error("listen", "err", err.Error())
		os.Exit(1)
	}
	slog.Info("listening", "addr", pc.LocalAddr().String())

	zone := map[string]net.IP{
		"example.local":       net.ParseIP("10.0.0.1"),
		"hello.example.local": net.ParseIP("10.0.0.2"),
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	srv := NewServer(pc, zone)
	go func() {
		<-ctx.Done()
		_ = pc.Close()
	}()
	_ = srv.Serve()
}
