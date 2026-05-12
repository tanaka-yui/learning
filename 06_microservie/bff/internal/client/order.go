package client

import (
	"context"

	orderv1 "microservie/proto/gen/go/order/v1"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Order struct{ c orderv1.OrderServiceClient }

func DialOrder(addr string) (*Order, error) {
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
	if err != nil {
		return nil, err
	}
	return &Order{c: orderv1.NewOrderServiceClient(conn)}, nil
}

func (o *Order) PlaceOrder(ctx context.Context, userID string, items []*orderv1.PlaceOrderItem) (*orderv1.PlaceOrderResponse, error) {
	return o.c.PlaceOrder(ctx, &orderv1.PlaceOrderRequest{UserId: userID, Items: items})
}

func (o *Order) GetOrder(ctx context.Context, orderID, userID string) (*orderv1.Order, error) {
	res, err := o.c.GetOrder(ctx, &orderv1.GetOrderRequest{Id: orderID, UserId: userID})
	if err != nil {
		return nil, err
	}
	return res.Order, nil
}

func (o *Order) ListOrders(ctx context.Context, userID string) ([]*orderv1.Order, error) {
	res, err := o.c.ListOrders(ctx, &orderv1.ListOrdersRequest{UserId: userID})
	if err != nil {
		return nil, err
	}
	return res.Orders, nil
}
