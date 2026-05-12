package client

import (
	"context"

	paymentv1 "microservie/proto/gen/go/payment/v1"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Payment is a gRPC client for the payment service that satisfies saga.Payment.
type Payment struct{ c paymentv1.PaymentServiceClient }

// DialPayment dials the payment service at addr and returns a Payment client.
func DialPayment(addr string) (*Payment, error) {
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
	if err != nil {
		return nil, err
	}
	return &Payment{c: paymentv1.NewPaymentServiceClient(conn)}, nil
}

func (p *Payment) Charge(ctx context.Context, idem, orderID string, amount int32) (string, error) {
	res, err := p.c.Charge(ctx, &paymentv1.ChargeRequest{
		IdempotencyKey: idem,
		OrderId:        orderID,
		AmountCents:    amount,
	})
	if err != nil {
		return "", err
	}
	return res.PaymentId, nil
}

func (p *Payment) Refund(ctx context.Context, paymentID string) error {
	_, err := p.c.Refund(ctx, &paymentv1.RefundRequest{PaymentId: paymentID})
	return err
}
