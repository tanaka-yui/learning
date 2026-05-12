package server

import (
	"context"
	"errors"

	"microservie/catalog/internal/repo"
	catalogv1 "microservie/proto/gen/go/catalog/v1"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Reader interface {
	List(ctx context.Context, limit, offset int32) ([]repo.Product, error)
	Get(ctx context.Context, id string) (repo.Product, error)
}

type Server struct {
	r Reader
}

func New(r Reader) *Server {
	return &Server{r: r}
}

func (s *Server) ListProducts(ctx context.Context, req *catalogv1.ListProductsRequest) (*catalogv1.ListProductsResponse, error) {
	limit := req.Limit
	if limit <= 0 {
		limit = 50
	}
	ps, err := s.r.List(ctx, limit, req.Offset)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	out := make([]*catalogv1.Product, 0, len(ps))
	for _, p := range ps {
		out = append(out, toProto(p))
	}
	return &catalogv1.ListProductsResponse{Products: out}, nil
}

func (s *Server) GetProduct(ctx context.Context, req *catalogv1.GetProductRequest) (*catalogv1.GetProductResponse, error) {
	p, err := s.r.Get(ctx, req.Id)
	if errors.Is(err, repo.ErrNotFound) {
		return nil, status.Error(codes.NotFound, "product not found")
	}
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &catalogv1.GetProductResponse{Product: toProto(p)}, nil
}

func toProto(p repo.Product) *catalogv1.Product {
	return &catalogv1.Product{
		Id:          p.ID,
		Name:        p.Name,
		Description: p.Description,
		PriceCents:  p.PriceCents,
	}
}
