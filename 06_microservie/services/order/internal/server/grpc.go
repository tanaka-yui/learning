package server

import (
	"context"

	"microservie/order/internal/repo"
	orderv1 "microservie/proto/gen/go/order/v1"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type OrderRepo interface {
	Create(ctx context.Context, userID string, items []repo.OrderItem) (string, int32, error)
	UpdateStatus(ctx context.Context, orderID, status string) error
	Get(ctx context.Context, orderID, userID string) (repo.Order, error)
	List(ctx context.Context, userID string) ([]repo.Order, error)
	LogStep(ctx context.Context, orderID, step, status, detail string) error
}

type Server struct {
	r OrderRepo
}

func New(r OrderRepo) *Server { return &Server{r: r} }

// PlaceOrder skeleton — Task 5 will add Saga.
// For now: Create the order row, leave status=PENDING, return that.
func (s *Server) PlaceOrder(ctx context.Context, req *orderv1.PlaceOrderRequest) (*orderv1.PlaceOrderResponse, error) {
	items := make([]repo.OrderItem, 0, len(req.Items))
	for _, it := range req.Items {
		items = append(items, repo.OrderItem{ProductID: it.ProductId, Quantity: it.Quantity, UnitPriceCents: it.UnitPriceCents})
	}
	orderID, _, err := s.r.Create(ctx, req.UserId, items)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &orderv1.PlaceOrderResponse{OrderId: orderID, Status: "PENDING"}, nil
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
