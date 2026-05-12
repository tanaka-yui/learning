package client

import (
	"context"
	"time"

	"microservie/order/internal/saga"
	inventoryv1 "microservie/proto/gen/go/inventory/v1"

	"github.com/cenkalti/backoff/v4"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// Inventory is a gRPC client for the inventory service that satisfies saga.Inventory.
type Inventory struct{ c inventoryv1.InventoryServiceClient }

// DialInventory dials the inventory service at addr and returns an Inventory client.
func DialInventory(addr string) (*Inventory, error) {
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
	if err != nil {
		return nil, err
	}
	return &Inventory{c: inventoryv1.NewInventoryServiceClient(conn)}, nil
}

func (i *Inventory) Reserve(ctx context.Context, orderID string, items []saga.Item) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	pItems := make([]*inventoryv1.Item, 0, len(items))
	for _, it := range items {
		pItems = append(pItems, &inventoryv1.Item{ProductId: it.ProductID, Quantity: it.Quantity})
	}

	var resID string
	op := func() error {
		r, err := i.c.Reserve(ctx, &inventoryv1.ReserveRequest{OrderId: orderID, Items: pItems})
		if err != nil {
			// 在庫不足は retry しない (Permanent でループ停止)
			if status.Code(err) == codes.FailedPrecondition {
				return backoff.Permanent(err)
			}
			return err
		}
		resID = r.ReservationId
		return nil
	}

	bo := backoff.WithContext(
		backoff.WithMaxRetries(backoff.NewExponentialBackOff(), 3),
		ctx,
	)
	if err := backoff.Retry(op, bo); err != nil {
		return "", err
	}
	return resID, nil
}

func (i *Inventory) Commit(ctx context.Context, resID string) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	_, err := i.c.Commit(ctx, &inventoryv1.CommitRequest{ReservationId: resID})
	return err
}

func (i *Inventory) Release(ctx context.Context, resID string) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	_, err := i.c.Release(ctx, &inventoryv1.ReleaseRequest{ReservationId: resID})
	return err
}
