package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"microservie/user-auth/internal/jwt"
	"microservie/user-auth/internal/obs"
	"microservie/user-auth/internal/repo"
	"microservie/user-auth/internal/server"
	userv1 "microservie/proto/gen/go/user/v1"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger.With("service", "user-auth"))

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	shutdownTracer, err := obs.InitTracing(ctx, "user-auth")
	if err != nil {
		slog.Error("init tracing", "err", err)
		os.Exit(1)
	}
	defer func() { _ = shutdownTracer(context.Background()) }()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		slog.Error("DATABASE_URL required")
		os.Exit(1)
	}
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		slog.Error("pgxpool", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := applyMigrations(ctx, pool); err != nil {
		slog.Error("migrate", "err", err)
		os.Exit(1)
	}

	secret := os.Getenv("JWT_SECRET")
	if len(secret) < 32 {
		slog.Error("JWT_SECRET must be >= 32 chars")
		os.Exit(1)
	}
	mgr := jwt.New([]byte(secret), 24*time.Hour)

	port := os.Getenv("GRPC_PORT")
	if port == "" {
		port = "50052"
	}
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		slog.Error("listen", "err", err)
		os.Exit(1)
	}

	gs := grpc.NewServer(grpc.StatsHandler(otelgrpc.NewServerHandler()))
	userv1.RegisterUserServiceServer(gs, server.New(repo.New(pool), mgr))

	go func() {
		<-ctx.Done()
		gs.GracefulStop()
	}()

	slog.Info("user-auth gRPC server starting", "port", port)
	if err := gs.Serve(lis); err != nil {
		slog.Error("serve", "err", err)
		os.Exit(1)
	}
}

func applyMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	for _, p := range []string{"/app/migrations/001_create_users.sql", "migrations/001_create_users.sql"} {
		if b, err := os.ReadFile(p); err == nil {
			_, err = pool.Exec(ctx, string(b))
			return err
		}
	}
	return os.ErrNotExist
}
