package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"

	catalogv1 "microservie/proto/gen/go/catalog/v1"

	"google.golang.org/grpc"
)

type stubServer struct{}

func (s *stubServer) ListProducts(ctx context.Context, req *catalogv1.ListProductsRequest) (*catalogv1.ListProductsResponse, error) {
	return &catalogv1.ListProductsResponse{Products: []*catalogv1.Product{}}, nil
}

func (s *stubServer) GetProduct(ctx context.Context, req *catalogv1.GetProductRequest) (*catalogv1.GetProductResponse, error) {
	return &catalogv1.GetProductResponse{}, nil
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	port := "50051"
	if v := os.Getenv("GRPC_PORT"); v != "" {
		port = v
	}

	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		slog.Error("listen failed", "err", err)
		os.Exit(1)
	}

	s := grpc.NewServer()
	catalogv1.RegisterCatalogServiceServer(s, &stubServer{})

	slog.Info("catalog gRPC server starting", "port", port)
	if err := s.Serve(lis); err != nil {
		slog.Error("serve failed", "err", err)
		os.Exit(1)
	}
	fmt.Println("shutting down")
}
