package main

import (
	"context"
	"log/slog"
	"net"
	"os"

	"microservie/catalog/internal/repo"
	"microservie/catalog/internal/server"
	catalogv1 "microservie/proto/gen/go/catalog/v1"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger.With("service", "catalog"))

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		slog.Error("DATABASE_URL is required")
		os.Exit(1)
	}

	ctx := context.Background()
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

	port := os.Getenv("GRPC_PORT")
	if port == "" {
		port = "50051"
	}
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		slog.Error("listen", "err", err)
		os.Exit(1)
	}

	gs := grpc.NewServer()
	catalogv1.RegisterCatalogServiceServer(gs, server.New(repo.New(pool)))

	slog.Info("catalog gRPC server starting", "port", port)
	if err := gs.Serve(lis); err != nil {
		slog.Error("serve", "err", err)
		os.Exit(1)
	}
}

func applyMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	b, err := os.ReadFile("/app/migrations/001_create_products.sql")
	if err != nil {
		b, err = os.ReadFile("migrations/001_create_products.sql")
		if err != nil {
			return err
		}
	}
	_, err = pool.Exec(ctx, string(b))
	return err
}
