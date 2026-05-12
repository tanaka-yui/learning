package repo_test

import (
	"context"
	"os"
	"testing"
	"time"

	"microservie/payment/internal/repo"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func setupDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	pg, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("payment"),
		postgres.WithUsername("test"), postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(wait.ForAll(
			wait.ForLog("database system is ready to accept connections").WithOccurrence(2),
			wait.ForListeningPort("5432/tcp"),
		).WithStartupTimeout(30*time.Second)),
	)
	if err != nil {
		t.Fatalf("postgres: %v", err)
	}
	t.Cleanup(func() { _ = pg.Terminate(ctx) })
	connStr, _ := pg.ConnectionString(ctx, "sslmode=disable")
	pool, _ := pgxpool.New(ctx, connStr)
	t.Cleanup(pool.Close)
	sql, _ := os.ReadFile("../../migrations/001_create_payments.sql")
	if _, err := pool.Exec(ctx, string(sql)); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return pool
}

func TestCreate_thenGetByIdempotencyKey(t *testing.T) {
	r := repo.New(setupDB(t))
	ctx := context.Background()

	id, err := r.Create(ctx, "idem-key-1", "order-1", 1000, "succeeded")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	got, err := r.GetByIdempotencyKey(ctx, "idem-key-1")
	if err != nil {
		t.Fatalf("GetByIdempotencyKey: %v", err)
	}
	if got.ID != id || got.IdempotencyKey != "idem-key-1" || got.OrderID != "order-1" ||
		got.AmountCents != 1000 || got.Status != "succeeded" {
		t.Fatalf("unexpected: %+v", got)
	}
}

func TestCreate_duplicateIdempotencyKeyReturnsExisting(t *testing.T) {
	r := repo.New(setupDB(t))
	ctx := context.Background()

	_, err := r.Create(ctx, "idem-key-dup", "order-2", 500, "succeeded")
	if err != nil {
		t.Fatalf("first Create: %v", err)
	}
	_, err = r.Create(ctx, "idem-key-dup", "order-3", 600, "succeeded")
	if err != repo.ErrIdempotencyKeyConflict {
		t.Fatalf("want ErrIdempotencyKeyConflict, got %v", err)
	}
}

func TestMarkRefunded_changesStatus(t *testing.T) {
	r := repo.New(setupDB(t))
	ctx := context.Background()

	id, err := r.Create(ctx, "idem-key-refund", "order-4", 2000, "succeeded")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := r.MarkRefunded(ctx, id); err != nil {
		t.Fatalf("MarkRefunded: %v", err)
	}
	got, err := r.GetByIdempotencyKey(ctx, "idem-key-refund")
	if err != nil {
		t.Fatalf("GetByIdempotencyKey: %v", err)
	}
	if got.Status != "refunded" {
		t.Fatalf("want status=refunded, got %q", got.Status)
	}
}
