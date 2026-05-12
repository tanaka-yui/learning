package repo_test

import (
	"context"
	"os"
	"testing"
	"time"

	"microservie/catalog/internal/repo"

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
		postgres.WithDatabase("catalog"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForAll(
				wait.ForLog("database system is ready to accept connections").WithOccurrence(2),
				wait.ForListeningPort("5432/tcp"),
			).WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("postgres start: %v", err)
	}
	t.Cleanup(func() { _ = pg.Terminate(ctx) })

	connStr, err := pg.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("connstr: %v", err)
	}

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)

	sql, err := os.ReadFile("../../migrations/001_create_products.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	if _, err := pool.Exec(ctx, string(sql)); err != nil {
		t.Fatalf("apply migration: %v", err)
	}
	return pool
}

func TestList_empty(t *testing.T) {
	pool := setupDB(t)
	r := repo.New(pool)

	got, err := r.List(context.Background(), 10, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("want 0 products, got %d", len(got))
	}
}

func TestInsert_then_Get(t *testing.T) {
	pool := setupDB(t)
	r := repo.New(pool)
	ctx := context.Background()

	p := repo.Product{ID: "p1", Name: "Pen", Description: "blue", PriceCents: 200}
	if err := r.Insert(ctx, p); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	got, err := r.Get(ctx, "p1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != "Pen" || got.PriceCents != 200 {
		t.Fatalf("unexpected product: %+v", got)
	}
}

func TestGet_notFound(t *testing.T) {
	pool := setupDB(t)
	r := repo.New(pool)

	_, err := r.Get(context.Background(), "missing")
	if err != repo.ErrNotFound {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}
