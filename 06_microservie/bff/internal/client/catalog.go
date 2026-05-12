package client

import (
	"context"

	catalogv1 "microservie/proto/gen/go/catalog/v1"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Catalog struct {
	c catalogv1.CatalogServiceClient
}

func DialCatalog(addr string) (*Catalog, error) {
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
	if err != nil {
		return nil, err
	}
	return &Catalog{c: catalogv1.NewCatalogServiceClient(conn)}, nil
}

func (c *Catalog) ListProducts(ctx context.Context) ([]*catalogv1.Product, error) {
	res, err := c.c.ListProducts(ctx, &catalogv1.ListProductsRequest{Limit: 50})
	if err != nil {
		return nil, err
	}
	return res.Products, nil
}
