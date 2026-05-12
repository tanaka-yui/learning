package repo_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"microservie/user-auth/internal/repo"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func setupDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	pg, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("auth"),
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
	sql, _ := os.ReadFile("../../migrations/001_create_users.sql")
	if _, err := pool.Exec(ctx, string(sql)); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return pool
}

func TestCreate_thenFindByEmail(t *testing.T) {
	r := repo.New(setupDB(t))
	ctx := context.Background()

	uid, err := r.Create(ctx, "a@example.com", "hashed-password")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	got, err := r.FindByEmail(ctx, "a@example.com")
	if err != nil {
		t.Fatalf("FindByEmail: %v", err)
	}
	if got.ID != uid || got.PasswordHash != "hashed-password" {
		t.Fatalf("unexpected: %+v", got)
	}
}

func TestCreate_duplicateEmailFails(t *testing.T) {
	r := repo.New(setupDB(t))
	ctx := context.Background()
	_, _ = r.Create(ctx, "a@example.com", "h1")
	_, err := r.Create(ctx, "a@example.com", "h2")
	if err != repo.ErrDuplicateEmail {
		t.Fatalf("want ErrDuplicateEmail, got %v", err)
	}
}

func TestFindByEmail_notFound(t *testing.T) {
	r := repo.New(setupDB(t))
	_, err := r.FindByEmail(context.Background(), "missing@example.com")
	if err != repo.ErrUserNotFound {
		t.Fatalf("want ErrUserNotFound, got %v", err)
	}
}

func TestRepoFindByID(t *testing.T) {
	ctx := context.Background()
	pool := setupDB(t)
	r := repo.New(pool)

	id, err := r.Create(ctx, "find-by-id@example.com", "hash")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	u, err := r.FindByID(ctx, id)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if u.Email != "find-by-id@example.com" {
		t.Errorf("email = %q, want find-by-id@example.com", u.Email)
	}

	_, err = r.FindByID(ctx, "00000000-0000-0000-0000-000000000000")
	if !errors.Is(err, repo.ErrUserNotFound) {
		t.Errorf("expected ErrUserNotFound, got %v", err)
	}
}
