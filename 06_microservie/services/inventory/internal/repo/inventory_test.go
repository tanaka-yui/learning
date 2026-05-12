package repo_test

import (
	"context"
	"os"
	"testing"
	"time"

	"microservie/inventory/internal/repo"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func setupDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()

	pg, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("inventory"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(wait.ForAll(
			wait.ForLog("database system is ready to accept connections").WithOccurrence(2),
			wait.ForListeningPort("5432/tcp"),
		).WithStartupTimeout(30*time.Second)),
	)
	if err != nil {
		t.Fatalf("postgres start: %v", err)
	}
	t.Cleanup(func() { _ = pg.Terminate(ctx) })

	connStr, _ := pg.ConnectionString(ctx, "sslmode=disable")
	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)

	sql, _ := os.ReadFile("../../migrations/001_create_inventory.sql")
	if _, err := pool.Exec(ctx, string(sql)); err != nil {
		t.Fatalf("apply migration: %v", err)
	}

	// 初期在庫
	_, _ = pool.Exec(ctx, "INSERT INTO stocks (product_id, available) VALUES ($1, $2)", "p-001", 100)
	return pool
}

func TestReserve_decrementsAvailableIncrementsReserved(t *testing.T) {
	pool := setupDB(t)
	r := repo.New(pool)
	ctx := context.Background()

	resID, err := r.Reserve(ctx, "order-1", []repo.Item{{ProductID: "p-001", Quantity: 3}})
	if err != nil {
		t.Fatalf("Reserve: %v", err)
	}
	if resID == "" {
		t.Fatal("want non-empty reservation_id")
	}

	stock, err := r.GetStock(ctx, "p-001")
	if err != nil {
		t.Fatalf("GetStock: %v", err)
	}
	if stock.Available != 97 || stock.Reserved != 3 {
		t.Fatalf("want available=97 reserved=3, got %+v", stock)
	}
}

func TestReserve_failsOnInsufficient(t *testing.T) {
	pool := setupDB(t)
	r := repo.New(pool)

	_, err := r.Reserve(context.Background(), "order-1",
		[]repo.Item{{ProductID: "p-001", Quantity: 9999}})
	if err != repo.ErrInsufficientStock {
		t.Fatalf("want ErrInsufficientStock, got %v", err)
	}
}

func TestReserve_thenCommit_drainsReservedNotAvailable(t *testing.T) {
	pool := setupDB(t)
	r := repo.New(pool)
	ctx := context.Background()

	resID, _ := r.Reserve(ctx, "order-1", []repo.Item{{ProductID: "p-001", Quantity: 5}})
	if err := r.Commit(ctx, resID); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	stock, _ := r.GetStock(ctx, "p-001")
	if stock.Available != 95 || stock.Reserved != 0 {
		t.Fatalf("want available=95 reserved=0, got %+v", stock)
	}
}

func TestReserve_thenRelease_restoresAvailable(t *testing.T) {
	pool := setupDB(t)
	r := repo.New(pool)
	ctx := context.Background()

	resID, _ := r.Reserve(ctx, "order-1", []repo.Item{{ProductID: "p-001", Quantity: 7}})
	if err := r.Release(ctx, resID); err != nil {
		t.Fatalf("Release: %v", err)
	}

	stock, _ := r.GetStock(ctx, "p-001")
	if stock.Available != 100 || stock.Reserved != 0 {
		t.Fatalf("want available=100 reserved=0, got %+v", stock)
	}
}

func TestCommit_idempotent(t *testing.T) {
	pool := setupDB(t)
	r := repo.New(pool)
	ctx := context.Background()

	resID, _ := r.Reserve(ctx, "order-1", []repo.Item{{ProductID: "p-001", Quantity: 2}})
	_ = r.Commit(ctx, resID)
	// 2回目の Commit はエラーにならない
	if err := r.Commit(ctx, resID); err != nil {
		t.Fatalf("Commit (2nd): %v", err)
	}
	stock, _ := r.GetStock(ctx, "p-001")
	if stock.Available != 98 {
		t.Fatalf("want available=98, got %d", stock.Available)
	}
}
