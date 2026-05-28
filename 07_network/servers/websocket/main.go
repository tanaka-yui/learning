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
	addr := flag.String("addr", ":9005", "listen address")
	flag.Parse()

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		slog.Error("listen", "err", err.Error())
		os.Exit(1)
	}
	slog.Info("listening", "addr", ln.Addr().String())

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	srv := NewServer(ln)
	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()
	_ = srv.Serve()
}
