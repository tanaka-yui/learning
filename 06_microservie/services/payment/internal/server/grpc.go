package server

import (
	"context"
	"errors"

	"microservie/payment/internal/flake"
	"microservie/payment/internal/repo"
	paymentv1 "microservie/proto/gen/go/payment/v1"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type PaymentRepo interface {
	Create(ctx context.Context, idemKey, orderID string, amount int32, statusStr string) (string, error)
	GetByIdempotencyKey(ctx context.Context, key string) (repo.Payment, error)
	MarkRefunded(ctx context.Context, id string) error
}

type Server struct {
	r     PaymentRepo
	flake *flake.Flake
}

func New(r PaymentRepo, f *flake.Flake) *Server { return &Server{r: r, flake: f} }

func (s *Server) Charge(ctx context.Context, req *paymentv1.ChargeRequest) (*paymentv1.ChargeResponse, error) {
	// 冪等性: 同じ idempotency_key で既に成功していればそれを返す
	if existing, err := s.r.GetByIdempotencyKey(ctx, req.IdempotencyKey); err == nil {
		return &paymentv1.ChargeResponse{PaymentId: existing.ID, Status: existing.Status}, nil
	} else if !errors.Is(err, repo.ErrPaymentNotFound) {
		return nil, status.Error(codes.Internal, err.Error())
	}

	if s.flake.ShouldFail() {
		return nil, status.Error(codes.Unavailable, "simulated payment processor failure")
	}

	id, err := s.r.Create(ctx, req.IdempotencyKey, req.OrderId, req.AmountCents, "succeeded")
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &paymentv1.ChargeResponse{PaymentId: id, Status: "succeeded"}, nil
}

func (s *Server) Refund(ctx context.Context, req *paymentv1.RefundRequest) (*paymentv1.RefundResponse, error) {
	if err := s.r.MarkRefunded(ctx, req.PaymentId); err != nil {
		if errors.Is(err, repo.ErrPaymentNotFound) {
			return nil, status.Error(codes.NotFound, "payment not found")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &paymentv1.RefundResponse{Status: "refunded"}, nil
}
