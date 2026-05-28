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
	addr := flag.String("addr", ":9002", "listen address")
	flag.Parse()

	pc, err := net.ListenPacket("udp", *addr)
	if err != nil {
		slog.Error("listen", "err", err.Error())
		os.Exit(1)
	}
	slog.Info("listening", "addr", pc.LocalAddr().String())

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	srv := NewServer(pc)
	go func() {
		<-ctx.Done()
		_ = pc.Close()
	}()
	_ = srv.Serve()
}
