package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"microservie/payment/internal/flake"
	"microservie/payment/internal/obs"
	"microservie/payment/internal/repo"
	"microservie/payment/internal/server"
	paymentv1 "microservie/proto/gen/go/payment/v1"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger.With("service", "payment"))

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	shutdownTracer, err := obs.InitTracing(ctx, "payment")
	if err != nil {
		slog.Error("init tracing", "err", err)
		os.Exit(1)
	}
	defer func() { _ = shutdownTracer(context.Background()) }()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		slog.Error("DATABASE_URL is required")
		os.Exit(1)
	}
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		slog.Error("pgxpool.New", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := applyMigrations(ctx, pool); err != nil {
		slog.Error("migrate", "err", err)
		os.Exit(1)
	}

	rateStr := os.Getenv("FLAKE_RATE")
	rate, _ := strconv.ParseFloat(rateStr, 64)
	fl := flake.New(rate, time.Now().UnixNano())

	port := os.Getenv("GRPC_PORT")
	if port == "" {
		port = "50055"
	}
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		slog.Error("listen", "err", err)
		os.Exit(1)
	}

	gs := grpc.NewServer(grpc.StatsHandler(otelgrpc.NewServerHandler()))
	paymentv1.RegisterPaymentServiceServer(gs, server.New(repo.New(pool), fl))

	go func() {
		<-ctx.Done()
		gs.GracefulStop()
	}()

	slog.Info("payment gRPC server starting", "port", port)
	if err := gs.Serve(lis); err != nil {
		slog.Error("serve", "err", err)
		os.Exit(1)
	}
}

func applyMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	for _, p := range []string{"/app/migrations/001_create_payments.sql", "migrations/001_create_payments.sql"} {
		if b, err := os.ReadFile(p); err == nil {
			_, err = pool.Exec(ctx, string(b))
			return err
		}
	}
	return os.ErrNotExist
}
