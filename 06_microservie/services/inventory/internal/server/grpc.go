package server

import (
	"context"
	"errors"

	"microservie/inventory/internal/repo"
	inventoryv1 "microservie/proto/gen/go/inventory/v1"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type InventoryRepo interface {
	Reserve(ctx context.Context, orderID string, items []repo.Item) (string, error)
	Commit(ctx context.Context, reservationID string) error
	Release(ctx context.Context, reservationID string) error
	GetStock(ctx context.Context, productID string) (repo.Stock, error)
}

type Server struct{ r InventoryRepo }

func New(r InventoryRepo) *Server { return &Server{r: r} }

func (s *Server) Reserve(ctx context.Context, req *inventoryv1.ReserveRequest) (*inventoryv1.ReserveResponse, error) {
	items := make([]repo.Item, 0, len(req.Items))
	for _, it := range req.Items {
		items = append(items, repo.Item{ProductID: it.ProductId, Quantity: it.Quantity})
	}
	id, err := s.r.Reserve(ctx, req.OrderId, items)
	if errors.Is(err, repo.ErrInsufficientStock) {
		return nil, status.Error(codes.FailedPrecondition, "insufficient stock")
	}
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &inventoryv1.ReserveResponse{ReservationId: id}, nil
}

func (s *Server) Commit(ctx context.Context, req *inventoryv1.CommitRequest) (*inventoryv1.CommitResponse, error) {
	if err := s.r.Commit(ctx, req.ReservationId); err != nil {
		if errors.Is(err, repo.ErrReservationNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &inventoryv1.CommitResponse{}, nil
}

func (s *Server) Release(ctx context.Context, req *inventoryv1.ReleaseRequest) (*inventoryv1.ReleaseResponse, error) {
	if err := s.r.Release(ctx, req.ReservationId); err != nil {
		if errors.Is(err, repo.ErrReservationNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &inventoryv1.ReleaseResponse{}, nil
}

func (s *Server) GetStock(ctx context.Context, req *inventoryv1.GetStockRequest) (*inventoryv1.GetStockResponse, error) {
	st, err := s.r.GetStock(ctx, req.ProductId)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &inventoryv1.GetStockResponse{Available: st.Available, Reserved: st.Reserved}, nil
}
