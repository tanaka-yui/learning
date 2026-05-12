package repo_test

import (
	"context"
	"os"
	"testing"
	"time"

	"microservie/order/internal/repo"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func setupDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	pg, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("orders"),
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
	sql, _ := os.ReadFile("../../migrations/001_create_orders.sql")
	if _, err := pool.Exec(ctx, string(sql)); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return pool
}

func TestCreate_thenGet(t *testing.T) {
	r := repo.New(setupDB(t))
	ctx := context.Background()

	items := []repo.OrderItem{
		{ProductID: "prod-1", Quantity: 2, UnitPriceCents: 500},
		{ProductID: "prod-2", Quantity: 1, UnitPriceCents: 1000},
	}
	orderID, totalCents, err := r.Create(ctx, "user-1", items)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if orderID == "" {
		t.Fatal("want non-empty orderID")
	}
	// total = 2*500 + 1*1000 = 2000
	if totalCents != 2000 {
		t.Fatalf("want totalCents=2000, got %d", totalCents)
	}

	got, err := r.Get(ctx, orderID, "user-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != orderID {
		t.Fatalf("want ID=%q, got %q", orderID, got.ID)
	}
	if got.UserID != "user-1" {
		t.Fatalf("want UserID=user-1, got %q", got.UserID)
	}
	if got.Status != "PENDING" {
		t.Fatalf("want Status=PENDING, got %q", got.Status)
	}
	if got.TotalCents != 2000 {
		t.Fatalf("want TotalCents=2000, got %d", got.TotalCents)
	}
	if len(got.Items) != 2 {
		t.Fatalf("want 2 items, got %d", len(got.Items))
	}
}

func TestUpdateStatus_changesStatus(t *testing.T) {
	r := repo.New(setupDB(t))
	ctx := context.Background()

	items := []repo.OrderItem{
		{ProductID: "prod-A", Quantity: 1, UnitPriceCents: 300},
	}
	orderID, _, err := r.Create(ctx, "user-2", items)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := r.UpdateStatus(ctx, orderID, "CONFIRMED"); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}

	got, err := r.Get(ctx, orderID, "user-2")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Status != "CONFIRMED" {
		t.Fatalf("want Status=CONFIRMED, got %q", got.Status)
	}
}

func TestGet_otherUserCantSee(t *testing.T) {
	r := repo.New(setupDB(t))
	ctx := context.Background()

	items := []repo.OrderItem{
		{ProductID: "prod-B", Quantity: 1, UnitPriceCents: 100},
	}
	orderID, _, err := r.Create(ctx, "user-1", items)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	_, err = r.Get(ctx, orderID, "user-2")
	if err != repo.ErrOrderNotFound {
		t.Fatalf("want ErrOrderNotFound, got %v", err)
	}
}

func TestLogStep_idempotent(t *testing.T) {
	pool := setupDB(t)
	r := repo.New(pool)
	ctx := context.Background()

	items := []repo.OrderItem{
		{ProductID: "prod-C", Quantity: 1, UnitPriceCents: 200},
	}
	orderID, _, err := r.Create(ctx, "user-3", items)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := r.LogStep(ctx, orderID, "reserve_inventory", "OK", "reserved"); err != nil {
		t.Fatalf("first LogStep: %v", err)
	}
	// Second call with same step — should not fail (upsert)
	if err := r.LogStep(ctx, orderID, "reserve_inventory", "OK", "reserved again"); err != nil {
		t.Fatalf("second LogStep: %v", err)
	}

	// Verify only one row exists for this step
	var count int
	err = pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM saga_log WHERE order_id=$1 AND step=$2`,
		orderID, "reserve_inventory",
	).Scan(&count)
	if err != nil {
		t.Fatalf("count query: %v", err)
	}
	if count != 1 {
		t.Fatalf("want 1 saga_log row, got %d", count)
	}
}
