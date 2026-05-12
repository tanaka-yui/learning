package server

import (
	"context"

	"microservie/order/internal/repo"
	"microservie/order/internal/saga"
	orderv1 "microservie/proto/gen/go/order/v1"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// OrderRepo is the full repository interface used by the server.
type OrderRepo interface {
	Create(ctx context.Context, userID string, items []repo.OrderItem) (string, int32, error)
	UpdateStatus(ctx context.Context, orderID, status string) error
	Get(ctx context.Context, orderID, userID string) (repo.Order, error)
	List(ctx context.Context, userID string) ([]repo.Order, error)
	LogStep(ctx context.Context, orderID, step, status, detail string) error
}

// SagaRunner abstracts the checkout saga for testing.
type SagaRunner interface {
	Run(ctx context.Context, in saga.Input) error
}

// Server is the gRPC server implementation for the order service.
type Server struct {
	r    OrderRepo
	saga SagaRunner
}

// New creates a new Server with the given repo and saga runner.
func New(r OrderRepo, sg SagaRunner) *Server { return &Server{r: r, saga: sg} }

// PlaceOrder creates an order and runs the checkout saga.
// On success the order is CONFIRMED; on saga failure it is FAILED.
func (s *Server) PlaceOrder(ctx context.Context, req *orderv1.PlaceOrderRequest) (*orderv1.PlaceOrderResponse, error) {
	items := make([]repo.OrderItem, 0, len(req.Items))
	for _, it := range req.Items {
		items = append(items, repo.OrderItem{ProductID: it.ProductId, Quantity: it.Quantity, UnitPriceCents: it.UnitPriceCents})
	}
	orderID, total, err := s.r.Create(ctx, req.UserId, items)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	sagaItems := make([]saga.Item, 0, len(items))
	for _, it := range items {
		sagaItems = append(sagaItems, saga.Item{ProductID: it.ProductID, Quantity: it.Quantity})
	}

	if err := s.saga.Run(ctx, saga.Input{OrderID: orderID, Items: sagaItems, TotalCents: total}); err != nil {
		// Saga called UpdateStatus("FAILED") internally.
		return &orderv1.PlaceOrderResponse{OrderId: orderID, Status: "FAILED"}, nil
	}
	return &orderv1.PlaceOrderResponse{OrderId: orderID, Status: "CONFIRMED"}, nil
}

func (s *Server) GetOrder(ctx context.Context, req *orderv1.GetOrderRequest) (*orderv1.GetOrderResponse, error) {
	o, err := s.r.Get(ctx, req.Id, req.UserId)
	if err == repo.ErrOrderNotFound {
		return nil, status.Error(codes.NotFound, "order not found")
	}
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &orderv1.GetOrderResponse{Order: toProto(o)}, nil
}

func (s *Server) ListOrders(ctx context.Context, req *orderv1.ListOrdersRequest) (*orderv1.ListOrdersResponse, error) {
	os, err := s.r.List(ctx, req.UserId)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	out := make([]*orderv1.Order, 0, len(os))
	for _, o := range os {
		out = append(out, toProto(o))
	}
	return &orderv1.ListOrdersResponse{Orders: out}, nil
}

func toProto(o repo.Order) *orderv1.Order {
	items := make([]*orderv1.OrderItem, 0, len(o.Items))
	for _, it := range o.Items {
		items = append(items, &orderv1.OrderItem{ProductId: it.ProductID, Quantity: it.Quantity, UnitPriceCents: it.UnitPriceCents})
	}
	return &orderv1.Order{
		Id: o.ID, UserId: o.UserID, Status: o.Status, TotalCents: o.TotalCents, Items: items,
	}
}
