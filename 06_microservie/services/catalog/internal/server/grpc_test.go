package server_test

import (
	"context"
	"net"
	"testing"

	"microservie/catalog/internal/repo"
	"microservie/catalog/internal/server"
	catalogv1 "microservie/proto/gen/go/catalog/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

type fakeRepo struct{ products []repo.Product }

func (f *fakeRepo) List(ctx context.Context, limit, offset int32) ([]repo.Product, error) {
	return f.products, nil
}
func (f *fakeRepo) Get(ctx context.Context, id string) (repo.Product, error) {
	for _, p := range f.products {
		if p.ID == id {
			return p, nil
		}
	}
	return repo.Product{}, repo.ErrNotFound
}

func dial(t *testing.T, s *grpc.Server) catalogv1.CatalogServiceClient {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	go func() { _ = s.Serve(lis) }()
	t.Cleanup(s.Stop)

	conn, err := grpc.NewClient("passthrough://bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return catalogv1.NewCatalogServiceClient(conn)
}

func TestListProducts_returnsAll(t *testing.T) {
	fr := &fakeRepo{products: []repo.Product{
		{ID: "a", Name: "A", PriceCents: 100},
		{ID: "b", Name: "B", PriceCents: 200},
	}}
	gs := grpc.NewServer()
	catalogv1.RegisterCatalogServiceServer(gs, server.New(fr))

	client := dial(t, gs)
	res, err := client.ListProducts(context.Background(), &catalogv1.ListProductsRequest{Limit: 10})
	if err != nil {
		t.Fatalf("ListProducts: %v", err)
	}
	if len(res.Products) != 2 {
		t.Fatalf("want 2 products, got %d", len(res.Products))
	}
}

func TestGetProduct_notFoundReturnsError(t *testing.T) {
	fr := &fakeRepo{}
	gs := grpc.NewServer()
	catalogv1.RegisterCatalogServiceServer(gs, server.New(fr))

	client := dial(t, gs)
	_, err := client.GetProduct(context.Background(), &catalogv1.GetProductRequest{Id: "missing"})
	if err == nil {
		t.Fatal("want error, got nil")
	}
}
