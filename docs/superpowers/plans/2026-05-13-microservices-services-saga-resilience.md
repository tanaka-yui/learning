# 06_microservie Plan 2: Services + Saga + Resilience Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans. Steps use `- [ ]` checkboxes.

**Goal:** Plan 1 の基盤の上に、残り4サービス（inventory / user-auth / payment / order）、Saga による checkout フロー、レジリエンスパターン（timeout / retry / circuit breaker）、BFF の認証＆注文系エンドポイントを実装する。

**Architecture:** 各サービスは Plan 1 の catalog と同じ縦断構造（proto + Postgres + gRPC + OTel + distroless Dockerfile）。order サービスがオーケストレータとして Saga を実装し、inventory.Reserve → payment.Charge → inventory.Commit を順に呼ぶ。失敗時は逆順に補償。BFF は JWT 認証ミドルウェア + 集約エンドポイントを提供する。

**Tech Stack:**
- 既存（Plan 1）: Go 1.25, gRPC, chi, pgx/v5, OTel + Jaeger, testcontainers-go
- 新規:
  - JWT: `github.com/golang-jwt/jwt/v5`
  - パスワードハッシュ: `golang.org/x/crypto/bcrypt`
  - Circuit Breaker: `github.com/sony/gobreaker`
  - リトライ: `github.com/cenkalti/backoff/v4`

**完了条件:**
1. `make up` で全 14 コンテナ（5 Postgres + 5 backend + bff + otel + jaeger + 既存 catalog 関連）が healthy
2. `make seed` で全サービスの初期データが投入される
3. `curl POST /api/auth/signin` → JWT を Cookie で受け取れる
4. `curl POST /api/checkout` で注文が確定し、Jaeger に `bff → order → inventory → payment → inventory.commit` の trace が出る
5. `make demo:retry` で `FLAKE_RATE=0.2` のとき注文の多くが最終的に成功する
6. `make demo:circuit` で `FLAKE_RATE=0.6` のとき Circuit Breaker が Open に遷移するログが出る
7. `make test` で全モジュール（catalog/inventory/user-auth/payment/order/bff）のテストがパス

**スコープ外（Plan 3 以降）:**
- React フロントエンド → Plan 3
- ドキュメント執筆 → Plan 4
- Postgres スパンの OTel 計装（pgx instrumentation）→ Plan 4 で扱うか、ここで足すか保留
- 結果整合性・非同期メッセージング・Outbox

---

## ファイル構成（新規分のみ）

```
06_microservie/
├── proto/
│   ├── inventory/v1/inventory.proto
│   ├── user/v1/user.proto
│   ├── payment/v1/payment.proto
│   └── order/v1/order.proto
├── services/
│   ├── inventory/    (proto/repo/server/obs/Dockerfile/migrations/seed)
│   ├── user-auth/    (+ internal/jwt/)
│   ├── payment/      (+ internal/flake/)
│   └── order/        (+ internal/saga/, internal/resilience/)
├── bff/
│   └── internal/
│       ├── handler/{auth.go,checkout.go,orders.go}
│       ├── middleware/auth.go
│       └── client/{user_auth.go,order.go}
├── docker-compose.yml   ← 拡張（4 Postgres + 4 services 追加）
├── Makefile             ← 拡張（demo:happy/retry/circuit）
└── VERIFICATION.md      ← 上書き更新
```

---

## 共通パターン（各サービスタスクで参照）

新規サービスは Plan 1 の catalog と同じ構造を取る。各タスクで明示的に同じパターンを繰り返す代わりに、ここを参照する。

### サービスを1つ追加するときの構成

1. `services/<name>/go.mod` — `module microservie/<name>`、`go 1.25`、必要な require
2. `services/<name>/migrations/*.sql` — テーブル DDL
3. `services/<name>/internal/repo/<name>.go` + `_test.go` — TDD で実装
4. `services/<name>/internal/server/grpc.go` + `_test.go` — TDD で実装、bufconn でテスト
5. `services/<name>/internal/obs/otel.go` — `services/catalog/internal/obs/otel.go` をそのままコピー（意図的重複）
6. `services/<name>/main.go` — Plan 1 の catalog/main.go を流用（DB URL / GRPC_PORT 環境変数 + OTel + GracefulStop）
7. `services/<name>/Dockerfile` — Plan 1 の catalog/Dockerfile をパス置換のみで流用
8. `services/<name>/seed/seed.sql` — 初期データ（必要に応じて）
9. `06_microservie/go.work` の `use (...)` ブロックに `./services/<name>` を追加
10. `docker-compose.yml` に postgres-<name> と <name> サービスを追加

**注:** 各サービスタスクの末尾ステップで `go.work` への追加を忘れないこと。

---

### Task 1: Inventory サービスを追加

**Files:**
- Create: `06_microservie/proto/inventory/v1/inventory.proto`
- Create: `06_microservie/services/inventory/go.mod`
- Create: `06_microservie/services/inventory/migrations/001_create_inventory.sql`
- Create: `06_microservie/services/inventory/internal/{repo,server,obs}/*.go` (with tests)
- Create: `06_microservie/services/inventory/main.go`
- Create: `06_microservie/services/inventory/Dockerfile`
- Create: `06_microservie/services/inventory/seed/seed.sql`
- Modify: `06_microservie/go.work` (add `./services/inventory`)

- [ ] **Step 1: proto を定義**

Create `06_microservie/proto/inventory/v1/inventory.proto`:

```proto
syntax = "proto3";

package inventory.v1;

service InventoryService {
  rpc Reserve(ReserveRequest) returns (ReserveResponse);
  rpc Commit(CommitRequest) returns (CommitResponse);
  rpc Release(ReleaseRequest) returns (ReleaseResponse);
  rpc GetStock(GetStockRequest) returns (GetStockResponse);
}

message Item {
  string product_id = 1;
  int32 quantity = 2;
}

message ReserveRequest {
  string order_id = 1;
  repeated Item items = 2;
}

message ReserveResponse {
  string reservation_id = 1;
}

message CommitRequest {
  string reservation_id = 1;
}

message CommitResponse {}

message ReleaseRequest {
  string reservation_id = 1;
}

message ReleaseResponse {}

message GetStockRequest {
  string product_id = 1;
}

message GetStockResponse {
  int32 available = 1;
  int32 reserved = 2;
}
```

- [ ] **Step 2: コード生成**

Run:
```bash
cd /Users/yui/Documents/workspace/tanaka-yui/learning/06_microservie
buf generate
```

Expected: `proto/gen/go/inventory/v1/inventory.pb.go` と `inventory_grpc.pb.go` が生成

- [ ] **Step 3: マイグレーション SQL**

Create `06_microservie/services/inventory/migrations/001_create_inventory.sql`:

```sql
CREATE TABLE IF NOT EXISTS stocks (
    product_id TEXT PRIMARY KEY,
    available  INTEGER NOT NULL CHECK (available >= 0),
    reserved   INTEGER NOT NULL DEFAULT 0 CHECK (reserved >= 0)
);

CREATE TABLE IF NOT EXISTS reservations (
    id         TEXT PRIMARY KEY,
    order_id   TEXT NOT NULL,
    status     TEXT NOT NULL CHECK (status IN ('held','committed','released')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS reservation_items (
    reservation_id TEXT NOT NULL REFERENCES reservations(id) ON DELETE CASCADE,
    product_id     TEXT NOT NULL,
    quantity       INTEGER NOT NULL CHECK (quantity > 0),
    PRIMARY KEY (reservation_id, product_id)
);
```

- [ ] **Step 4: go.mod**

Create `06_microservie/services/inventory/go.mod`:

```
module microservie/inventory

go 1.25

require (
    microservie/proto v0.0.0
    github.com/jackc/pgx/v5 v5.7.1
    github.com/google/uuid v1.6.0
    google.golang.org/grpc v1.66.0
)

replace microservie/proto => ../../proto
```

- [ ] **Step 5: go.work に追加**

Edit `06_microservie/go.work`:

```
go 1.25.0

use (
    ./proto
    ./services/catalog
    ./services/inventory
    ./bff
)
```

- [ ] **Step 6: repo のTDDテストを書く（失敗）**

Create `06_microservie/services/inventory/internal/repo/inventory_test.go`:

```go
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
```

- [ ] **Step 7: テスト失敗を確認**

Run:
```bash
cd /Users/yui/Documents/workspace/tanaka-yui/learning/06_microservie/services/inventory
go mod tidy
DOCKER_HOST=unix://$HOME/.rd/docker.sock TESTCONTAINERS_RYUK_DISABLED=true go test ./internal/repo/...
```

Expected: コンパイルエラー（`repo.New`, `repo.Item`, `repo.ErrInsufficientStock` 未定義）

- [ ] **Step 8: repo を実装**

Create `06_microservie/services/inventory/internal/repo/inventory.go`:

```go
package repo

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrInsufficientStock = errors.New("insufficient stock")
var ErrReservationNotFound = errors.New("reservation not found")

type Item struct {
	ProductID string
	Quantity  int32
}

type Stock struct {
	Available int32
	Reserved  int32
}

type Repo struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

// Reserve は items 分を available から差し引き、reserved に振り替える。
// 1つでも不足していれば失敗（全体ロールバック）。reservation_id を返す。
func (r *Repo) Reserve(ctx context.Context, orderID string, items []Item) (string, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)

	resID := uuid.NewString()
	if _, err := tx.Exec(ctx,
		`INSERT INTO reservations (id, order_id, status) VALUES ($1, $2, 'held')`,
		resID, orderID,
	); err != nil {
		return "", err
	}

	for _, it := range items {
		ct, err := tx.Exec(ctx,
			`UPDATE stocks SET available = available - $1, reserved = reserved + $1
			 WHERE product_id = $2 AND available >= $1`,
			it.Quantity, it.ProductID,
		)
		if err != nil {
			return "", err
		}
		if ct.RowsAffected() == 0 {
			return "", ErrInsufficientStock
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO reservation_items (reservation_id, product_id, quantity)
			 VALUES ($1, $2, $3)`,
			resID, it.ProductID, it.Quantity,
		); err != nil {
			return "", err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return resID, nil
}

// Commit は reserved を確定し、reservations を committed にする。冪等。
func (r *Repo) Commit(ctx context.Context, reservationID string) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var status string
	if err := tx.QueryRow(ctx,
		`SELECT status FROM reservations WHERE id = $1 FOR UPDATE`, reservationID,
	).Scan(&status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrReservationNotFound
		}
		return err
	}
	if status == "committed" {
		return tx.Commit(ctx) // 冪等
	}
	if status == "released" {
		return errors.New("reservation already released")
	}

	if _, err := tx.Exec(ctx,
		`UPDATE stocks s SET reserved = s.reserved - ri.quantity
		 FROM reservation_items ri
		 WHERE ri.reservation_id = $1 AND ri.product_id = s.product_id`,
		reservationID,
	); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`UPDATE reservations SET status = 'committed' WHERE id = $1`, reservationID,
	); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Release は reserved を available に戻し、reservations を released にする。冪等。
func (r *Repo) Release(ctx context.Context, reservationID string) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var status string
	if err := tx.QueryRow(ctx,
		`SELECT status FROM reservations WHERE id = $1 FOR UPDATE`, reservationID,
	).Scan(&status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrReservationNotFound
		}
		return err
	}
	if status == "released" {
		return tx.Commit(ctx) // 冪等
	}
	if status == "committed" {
		return errors.New("reservation already committed")
	}

	if _, err := tx.Exec(ctx,
		`UPDATE stocks s SET available = s.available + ri.quantity, reserved = s.reserved - ri.quantity
		 FROM reservation_items ri
		 WHERE ri.reservation_id = $1 AND ri.product_id = s.product_id`,
		reservationID,
	); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`UPDATE reservations SET status = 'released' WHERE id = $1`, reservationID,
	); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *Repo) GetStock(ctx context.Context, productID string) (Stock, error) {
	var s Stock
	err := r.pool.QueryRow(ctx,
		`SELECT available, reserved FROM stocks WHERE product_id = $1`, productID,
	).Scan(&s.Available, &s.Reserved)
	return s, err
}
```

- [ ] **Step 9: テスト合格を確認**

Run:
```bash
cd /Users/yui/Documents/workspace/tanaka-yui/learning/06_microservie/services/inventory
go mod tidy
DOCKER_HOST=unix://$HOME/.rd/docker.sock TESTCONTAINERS_RYUK_DISABLED=true go test ./internal/repo/...
```

Expected: 5 テスト全パス

- [ ] **Step 10: server 実装（TDD は最小限）**

Create `06_microservie/services/inventory/internal/server/grpc.go`:

```go
package server

import (
	"context"
	"errors"

	"microservie/inventory/internal/repo"
	inventoryv1 "microservie/proto/gen/go/inventory/v1"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type InventoryRepo interface {
	Reserve(ctx context.Context, orderID string, items []repo.Item) (string, error)
	Commit(ctx context.Context, reservationID string) error
	Release(ctx context.Context, reservationID string) error
	GetStock(ctx context.Context, productID string) (repo.Stock, error)
}

type Server struct{ r InventoryRepo }

func New(r InventoryRepo) *Server { return &Server{r: r} }

func (s *Server) Reserve(ctx context.Context, req *inventoryv1.ReserveRequest) (*inventoryv1.ReserveResponse, error) {
	items := make([]repo.Item, 0, len(req.Items))
	for _, it := range req.Items {
		items = append(items, repo.Item{ProductID: it.ProductId, Quantity: it.Quantity})
	}
	id, err := s.r.Reserve(ctx, req.OrderId, items)
	if errors.Is(err, repo.ErrInsufficientStock) {
		return nil, status.Error(codes.FailedPrecondition, "insufficient stock")
	}
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &inventoryv1.ReserveResponse{ReservationId: id}, nil
}

func (s *Server) Commit(ctx context.Context, req *inventoryv1.CommitRequest) (*inventoryv1.CommitResponse, error) {
	if err := s.r.Commit(ctx, req.ReservationId); err != nil {
		if errors.Is(err, repo.ErrReservationNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &inventoryv1.CommitResponse{}, nil
}

func (s *Server) Release(ctx context.Context, req *inventoryv1.ReleaseRequest) (*inventoryv1.ReleaseResponse, error) {
	if err := s.r.Release(ctx, req.ReservationId); err != nil {
		if errors.Is(err, repo.ErrReservationNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &inventoryv1.ReleaseResponse{}, nil
}

func (s *Server) GetStock(ctx context.Context, req *inventoryv1.GetStockRequest) (*inventoryv1.GetStockResponse, error) {
	st, err := s.r.GetStock(ctx, req.ProductId)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &inventoryv1.GetStockResponse{Available: st.Available, Reserved: st.Reserved}, nil
}
```

Test (smoke only) — Create `06_microservie/services/inventory/internal/server/grpc_test.go`:

```go
package server_test

import (
	"context"
	"testing"

	"microservie/inventory/internal/repo"
	"microservie/inventory/internal/server"
	inventoryv1 "microservie/proto/gen/go/inventory/v1"
)

type fakeRepo struct {
	resID string
	err   error
}

func (f *fakeRepo) Reserve(ctx context.Context, orderID string, items []repo.Item) (string, error) {
	return f.resID, f.err
}
func (f *fakeRepo) Commit(ctx context.Context, id string) error  { return f.err }
func (f *fakeRepo) Release(ctx context.Context, id string) error { return f.err }
func (f *fakeRepo) GetStock(ctx context.Context, pid string) (repo.Stock, error) {
	return repo.Stock{Available: 10}, nil
}

func TestReserve_OK(t *testing.T) {
	s := server.New(&fakeRepo{resID: "r-1"})
	res, err := s.Reserve(context.Background(), &inventoryv1.ReserveRequest{
		OrderId: "o-1", Items: []*inventoryv1.Item{{ProductId: "p-001", Quantity: 1}},
	})
	if err != nil || res.ReservationId != "r-1" {
		t.Fatalf("got %v, %v", res, err)
	}
}

func TestReserve_InsufficientReturnsFailedPrecondition(t *testing.T) {
	s := server.New(&fakeRepo{err: repo.ErrInsufficientStock})
	_, err := s.Reserve(context.Background(), &inventoryv1.ReserveRequest{OrderId: "o-1"})
	if err == nil {
		t.Fatal("want error")
	}
}
```

- [ ] **Step 11: obs/otel.go をコピー**

Run:
```bash
cp /Users/yui/Documents/workspace/tanaka-yui/learning/06_microservie/services/catalog/internal/obs/otel.go \
   /Users/yui/Documents/workspace/tanaka-yui/learning/06_microservie/services/inventory/internal/obs/otel.go
```

Then open the copied file and confirm import path `microservie/catalog/...` does NOT appear (the file only imports OTel SDK, not catalog packages). No edits needed.

- [ ] **Step 12: main.go**

Create `06_microservie/services/inventory/main.go` — pattern identical to `services/catalog/main.go`. Replace:
- `service` log attr `catalog` → `inventory`
- imports `microservie/catalog/...` → `microservie/inventory/...`
- proto import: `catalogv1` → `inventoryv1`
- gRPC register: `catalogv1.RegisterCatalogServiceServer` → `inventoryv1.RegisterInventoryServiceServer`
- default `GRPC_PORT` → `50054`
- migration path: `migrations/001_create_inventory.sql`
- `obs.InitTracing(ctx, "inventory")`

Full code:

```go
package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"microservie/inventory/internal/obs"
	"microservie/inventory/internal/repo"
	"microservie/inventory/internal/server"
	inventoryv1 "microservie/proto/gen/go/inventory/v1"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger.With("service", "inventory"))

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	shutdownTracer, err := obs.InitTracing(ctx, "inventory")
	if err != nil {
		slog.Error("init tracing", "err", err)
		os.Exit(1)
	}
	defer func() { _ = shutdownTracer(context.Background()) }()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		slog.Error("DATABASE_URL is required")
		os.Exit(1)
	}
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		slog.Error("pgxpool.New", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := applyMigrations(ctx, pool); err != nil {
		slog.Error("migrate", "err", err)
		os.Exit(1)
	}

	port := os.Getenv("GRPC_PORT")
	if port == "" {
		port = "50054"
	}
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		slog.Error("listen", "err", err)
		os.Exit(1)
	}

	gs := grpc.NewServer(grpc.StatsHandler(otelgrpc.NewServerHandler()))
	inventoryv1.RegisterInventoryServiceServer(gs, server.New(repo.New(pool)))

	go func() {
		<-ctx.Done()
		gs.GracefulStop()
	}()

	slog.Info("inventory gRPC server starting", "port", port)
	if err := gs.Serve(lis); err != nil {
		slog.Error("serve", "err", err)
		os.Exit(1)
	}
}

func applyMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	for _, p := range []string{"/app/migrations/001_create_inventory.sql", "migrations/001_create_inventory.sql"} {
		if b, err := os.ReadFile(p); err == nil {
			_, err = pool.Exec(ctx, string(b))
			return err
		}
	}
	return os.ErrNotExist
}
```

- [ ] **Step 13: Dockerfile**

Create `06_microservie/services/inventory/Dockerfile` — copy from catalog and replace `catalog` → `inventory`, port `50051` → `50054`:

```dockerfile
FROM golang:1.26-bookworm AS builder

WORKDIR /work

COPY proto/go.mod proto/go.sum proto/
COPY proto/gen ./proto/gen

WORKDIR /work/services/inventory
COPY services/inventory/go.mod services/inventory/go.sum ./
RUN go mod download

COPY services/inventory/ ./

RUN CGO_ENABLED=0 go build -o /out/inventory .

FROM gcr.io/distroless/static:nonroot
WORKDIR /app
COPY --from=builder /out/inventory /app/inventory
COPY services/inventory/migrations /app/migrations

ENV GRPC_PORT=50054
EXPOSE 50054

ENTRYPOINT ["/app/inventory"]
```

- [ ] **Step 14: seed.sql**

Create `06_microservie/services/inventory/seed/seed.sql`:

```sql
TRUNCATE TABLE reservation_items, reservations, stocks RESTART IDENTITY CASCADE;

INSERT INTO stocks (product_id, available) VALUES
  ('p-001', 100), ('p-002', 200), ('p-003', 30),  ('p-004', 150),
  ('p-005', 80),  ('p-006', 40),  ('p-007', 500), ('p-008', 60),
  ('p-009', 120), ('p-010', 250);
```

- [ ] **Step 15: ビルド確認**

Run:
```bash
cd /Users/yui/Documents/workspace/tanaka-yui/learning/06_microservie/services/inventory
go build -o /tmp/inventory-bin .
```

Expected: ビルド成功

- [ ] **Step 16: コミット**

```bash
cd /Users/yui/Documents/workspace/tanaka-yui/learning
git add 06_microservie/proto/inventory 06_microservie/services/inventory 06_microservie/go.work
git commit -m "microservices(inventory): add Reserve/Commit/Release gRPC service with TDD"
```

---

### Task 2: User-Auth サービスを追加（JWT 発行・検証付き）

**Files:**
- Create: `06_microservie/proto/user/v1/user.proto`
- Create: `06_microservie/services/user-auth/{go.mod, Dockerfile, main.go, migrations/001_*, internal/{repo,server,obs,jwt}/*, seed/*}`
- Modify: `06_microservie/go.work`

- [ ] **Step 1: proto を定義**

Create `06_microservie/proto/user/v1/user.proto`:

```proto
syntax = "proto3";

package user.v1;

service UserService {
  rpc SignUp(SignUpRequest) returns (SignUpResponse);
  rpc SignIn(SignInRequest) returns (SignInResponse);
  rpc ValidateToken(ValidateTokenRequest) returns (ValidateTokenResponse);
}

message SignUpRequest {
  string email = 1;
  string password = 2;
}

message SignUpResponse {
  string user_id = 1;
}

message SignInRequest {
  string email = 1;
  string password = 2;
}

message SignInResponse {
  string token = 1;
}

message ValidateTokenRequest {
  string token = 1;
}

message ValidateTokenResponse {
  string user_id = 1;
}
```

- [ ] **Step 2: コード生成**

Run: `cd 06_microservie && buf generate`

- [ ] **Step 3: マイグレーション**

Create `06_microservie/services/user-auth/migrations/001_create_users.sql`:

```sql
CREATE TABLE IF NOT EXISTS users (
    id            TEXT PRIMARY KEY,
    email         TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

- [ ] **Step 4: go.mod + go.work 追加**

`services/user-auth/go.mod`:

```
module microservie/user-auth

go 1.25

require (
    microservie/proto v0.0.0
    github.com/jackc/pgx/v5 v5.7.1
    github.com/google/uuid v1.6.0
    github.com/golang-jwt/jwt/v5 v5.2.1
    golang.org/x/crypto v0.27.0
    google.golang.org/grpc v1.66.0
)

replace microservie/proto => ../../proto
```

Edit `06_microservie/go.work`:

```
go 1.25.0

use (
    ./proto
    ./services/catalog
    ./services/inventory
    ./services/user-auth
    ./bff
)
```

- [ ] **Step 5: JWT パッケージの TDD**

Create `06_microservie/services/user-auth/internal/jwt/jwt_test.go`:

```go
package jwt_test

import (
	"testing"
	"time"

	"microservie/user-auth/internal/jwt"
)

func TestIssueAndVerify_roundTrip(t *testing.T) {
	mgr := jwt.New([]byte("test-secret-32-bytes-long-padding"), time.Hour)

	token, err := mgr.Issue("user-1")
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	uid, err := mgr.Verify(token)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if uid != "user-1" {
		t.Fatalf("want user-1, got %s", uid)
	}
}

func TestVerify_rejectsTamperedToken(t *testing.T) {
	mgr := jwt.New([]byte("test-secret-32-bytes-long-padding"), time.Hour)
	token, _ := mgr.Issue("user-1")
	_, err := mgr.Verify(token + "x")
	if err == nil {
		t.Fatal("want error on tampered token")
	}
}

func TestVerify_rejectsExpiredToken(t *testing.T) {
	mgr := jwt.New([]byte("test-secret-32-bytes-long-padding"), -1*time.Second)
	token, _ := mgr.Issue("user-1")
	_, err := mgr.Verify(token)
	if err == nil {
		t.Fatal("want error on expired token")
	}
}
```

Create `06_microservie/services/user-auth/internal/jwt/jwt.go`:

```go
package jwt

import (
	"errors"
	"time"

	jwtv5 "github.com/golang-jwt/jwt/v5"
)

var ErrInvalidToken = errors.New("invalid token")

type Manager struct {
	secret []byte
	ttl    time.Duration
}

func New(secret []byte, ttl time.Duration) *Manager {
	return &Manager{secret: secret, ttl: ttl}
}

func (m *Manager) Issue(userID string) (string, error) {
	now := time.Now()
	claims := jwtv5.MapClaims{
		"sub": userID,
		"iat": now.Unix(),
		"exp": now.Add(m.ttl).Unix(),
	}
	tok := jwtv5.NewWithClaims(jwtv5.SigningMethodHS256, claims)
	return tok.SignedString(m.secret)
}

func (m *Manager) Verify(tokenStr string) (string, error) {
	tok, err := jwtv5.Parse(tokenStr, func(t *jwtv5.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwtv5.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return m.secret, nil
	})
	if err != nil || !tok.Valid {
		return "", ErrInvalidToken
	}
	claims, ok := tok.Claims.(jwtv5.MapClaims)
	if !ok {
		return "", ErrInvalidToken
	}
	sub, _ := claims["sub"].(string)
	if sub == "" {
		return "", ErrInvalidToken
	}
	return sub, nil
}
```

Run tests:
```bash
cd 06_microservie/services/user-auth && go mod tidy && go test ./internal/jwt/...
```

Expected: 3 テストパス

- [ ] **Step 6: repo の TDD**

Create `06_microservie/services/user-auth/internal/repo/users_test.go`:

```go
package repo_test

import (
	"context"
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
```

Create `06_microservie/services/user-auth/internal/repo/users.go`:

```go
package repo

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrUserNotFound   = errors.New("user not found")
	ErrDuplicateEmail = errors.New("duplicate email")
)

type User struct {
	ID, Email, PasswordHash string
}

type Repo struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

func (r *Repo) Create(ctx context.Context, email, hash string) (string, error) {
	id := uuid.NewString()
	_, err := r.pool.Exec(ctx, `INSERT INTO users (id, email, password_hash) VALUES ($1,$2,$3)`, id, email, hash)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return "", ErrDuplicateEmail
	}
	if err != nil {
		return "", err
	}
	return id, nil
}

func (r *Repo) FindByEmail(ctx context.Context, email string) (User, error) {
	var u User
	err := r.pool.QueryRow(ctx, `SELECT id, email, password_hash FROM users WHERE email=$1`, email).
		Scan(&u.ID, &u.Email, &u.PasswordHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrUserNotFound
	}
	return u, err
}
```

Run tests:
```bash
DOCKER_HOST=unix://$HOME/.rd/docker.sock TESTCONTAINERS_RYUK_DISABLED=true \
go test ./internal/repo/...
```

Expected: 3 テストパス

- [ ] **Step 7: server 実装**

Create `06_microservie/services/user-auth/internal/server/grpc.go`:

```go
package server

import (
	"context"
	"errors"

	"microservie/user-auth/internal/jwt"
	"microservie/user-auth/internal/repo"
	userv1 "microservie/proto/gen/go/user/v1"

	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type UserRepo interface {
	Create(ctx context.Context, email, hash string) (string, error)
	FindByEmail(ctx context.Context, email string) (repo.User, error)
}

type Server struct {
	r   UserRepo
	jwt *jwt.Manager
}

func New(r UserRepo, j *jwt.Manager) *Server { return &Server{r: r, jwt: j} }

func (s *Server) SignUp(ctx context.Context, req *userv1.SignUpRequest) (*userv1.SignUpResponse, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	id, err := s.r.Create(ctx, req.Email, string(hash))
	if errors.Is(err, repo.ErrDuplicateEmail) {
		return nil, status.Error(codes.AlreadyExists, "email taken")
	}
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &userv1.SignUpResponse{UserId: id}, nil
}

func (s *Server) SignIn(ctx context.Context, req *userv1.SignInRequest) (*userv1.SignInResponse, error) {
	u, err := s.r.FindByEmail(ctx, req.Email)
	if errors.Is(err, repo.ErrUserNotFound) {
		return nil, status.Error(codes.Unauthenticated, "invalid credentials")
	}
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.Password)); err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid credentials")
	}
	token, err := s.jwt.Issue(u.ID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &userv1.SignInResponse{Token: token}, nil
}

func (s *Server) ValidateToken(ctx context.Context, req *userv1.ValidateTokenRequest) (*userv1.ValidateTokenResponse, error) {
	uid, err := s.jwt.Verify(req.Token)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid token")
	}
	return &userv1.ValidateTokenResponse{UserId: uid}, nil
}
```

- [ ] **Step 8: server のテスト**

Create `06_microservie/services/user-auth/internal/server/grpc_test.go`:

```go
package server_test

import (
	"context"
	"testing"
	"time"

	"microservie/user-auth/internal/jwt"
	"microservie/user-auth/internal/repo"
	"microservie/user-auth/internal/server"
	userv1 "microservie/proto/gen/go/user/v1"

	"golang.org/x/crypto/bcrypt"
)

type memRepo struct{ users map[string]repo.User }

func (m *memRepo) Create(ctx context.Context, email, hash string) (string, error) {
	if _, ok := m.users[email]; ok {
		return "", repo.ErrDuplicateEmail
	}
	id := "id-" + email
	m.users[email] = repo.User{ID: id, Email: email, PasswordHash: hash}
	return id, nil
}
func (m *memRepo) FindByEmail(ctx context.Context, email string) (repo.User, error) {
	u, ok := m.users[email]
	if !ok {
		return repo.User{}, repo.ErrUserNotFound
	}
	return u, nil
}

func TestSignUp_thenSignIn_returnsToken(t *testing.T) {
	r := &memRepo{users: map[string]repo.User{}}
	j := jwt.New([]byte("test-secret-32-bytes-long-padding"), time.Hour)
	s := server.New(r, j)

	_, err := s.SignUp(context.Background(), &userv1.SignUpRequest{Email: "a@x.com", Password: "pw"})
	if err != nil {
		t.Fatalf("SignUp: %v", err)
	}
	res, err := s.SignIn(context.Background(), &userv1.SignInRequest{Email: "a@x.com", Password: "pw"})
	if err != nil || res.Token == "" {
		t.Fatalf("SignIn: token=%q err=%v", res.GetToken(), err)
	}
}

func TestSignIn_wrongPasswordReturnsUnauthenticated(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("right"), bcrypt.DefaultCost)
	r := &memRepo{users: map[string]repo.User{"a@x.com": {ID: "u-1", Email: "a@x.com", PasswordHash: string(hash)}}}
	j := jwt.New([]byte("test-secret-32-bytes-long-padding"), time.Hour)
	s := server.New(r, j)

	_, err := s.SignIn(context.Background(), &userv1.SignInRequest{Email: "a@x.com", Password: "wrong"})
	if err == nil {
		t.Fatal("want error")
	}
}

func TestValidateToken_returnsUserID(t *testing.T) {
	j := jwt.New([]byte("test-secret-32-bytes-long-padding"), time.Hour)
	token, _ := j.Issue("user-42")
	s := server.New(&memRepo{}, j)

	res, err := s.ValidateToken(context.Background(), &userv1.ValidateTokenRequest{Token: token})
	if err != nil || res.UserId != "user-42" {
		t.Fatalf("res=%v err=%v", res, err)
	}
}
```

Run tests:
```bash
go test ./internal/server/...
```

Expected: 3 テストパス

- [ ] **Step 9: obs/otel.go コピー**

```bash
cp services/catalog/internal/obs/otel.go services/user-auth/internal/obs/otel.go
```

- [ ] **Step 10: main.go**

Create `06_microservie/services/user-auth/main.go`:

```go
package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"microservie/user-auth/internal/jwt"
	"microservie/user-auth/internal/obs"
	"microservie/user-auth/internal/repo"
	"microservie/user-auth/internal/server"
	userv1 "microservie/proto/gen/go/user/v1"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger.With("service", "user-auth"))

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	shutdownTracer, err := obs.InitTracing(ctx, "user-auth")
	if err != nil {
		slog.Error("init tracing", "err", err)
		os.Exit(1)
	}
	defer func() { _ = shutdownTracer(context.Background()) }()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		slog.Error("DATABASE_URL required")
		os.Exit(1)
	}
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		slog.Error("pgxpool", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := applyMigrations(ctx, pool); err != nil {
		slog.Error("migrate", "err", err)
		os.Exit(1)
	}

	secret := os.Getenv("JWT_SECRET")
	if len(secret) < 32 {
		slog.Error("JWT_SECRET must be >= 32 chars")
		os.Exit(1)
	}
	mgr := jwt.New([]byte(secret), 24*time.Hour)

	port := os.Getenv("GRPC_PORT")
	if port == "" {
		port = "50052"
	}
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		slog.Error("listen", "err", err)
		os.Exit(1)
	}

	gs := grpc.NewServer(grpc.StatsHandler(otelgrpc.NewServerHandler()))
	userv1.RegisterUserServiceServer(gs, server.New(repo.New(pool), mgr))

	go func() {
		<-ctx.Done()
		gs.GracefulStop()
	}()

	slog.Info("user-auth gRPC server starting", "port", port)
	if err := gs.Serve(lis); err != nil {
		slog.Error("serve", "err", err)
		os.Exit(1)
	}
}

func applyMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	for _, p := range []string{"/app/migrations/001_create_users.sql", "migrations/001_create_users.sql"} {
		if b, err := os.ReadFile(p); err == nil {
			_, err = pool.Exec(ctx, string(b))
			return err
		}
	}
	return os.ErrNotExist
}
```

- [ ] **Step 11: Dockerfile**

Same shape as catalog/inventory. Replace `inventory` → `user-auth` (directory has hyphen but Go bin can be `user-auth`), port → `50052`:

```dockerfile
FROM golang:1.26-bookworm AS builder
WORKDIR /work
COPY proto/go.mod proto/go.sum proto/
COPY proto/gen ./proto/gen
WORKDIR /work/services/user-auth
COPY services/user-auth/go.mod services/user-auth/go.sum ./
RUN go mod download
COPY services/user-auth/ ./
RUN CGO_ENABLED=0 go build -o /out/user-auth .

FROM gcr.io/distroless/static:nonroot
WORKDIR /app
COPY --from=builder /out/user-auth /app/user-auth
COPY services/user-auth/migrations /app/migrations
ENV GRPC_PORT=50052
EXPOSE 50052
ENTRYPOINT ["/app/user-auth"]
```

- [ ] **Step 12: seed.sql**

Create `06_microservie/services/user-auth/seed/seed.sql`:

```sql
TRUNCATE TABLE users;

-- パスワードは bcrypt('password') を事前計算したものを置く
-- alice@example.com / password、bob@example.com / password
INSERT INTO users (id, email, password_hash) VALUES
  ('u-001', 'alice@example.com', '$2a$10$JfL5R0kU8z3vH7lY0n9o.O5N1g/PgwL.Q3qkUjP3jD0Gz1f9hCkSe'),
  ('u-002', 'bob@example.com',   '$2a$10$JfL5R0kU8z3vH7lY0n9o.O5N1g/PgwL.Q3qkUjP3jD0Gz1f9hCkSe');
```

> 注: bcrypt ハッシュは Plan 2 実装時にビルダーが `bcrypt('password', 10)` を生成して置き換える。仮ハッシュなので最初の `make seed:user` で失敗する可能性がある。実装者は次のコマンドで実値を生成して上記の hash 列を差し替えること:
> ```bash
> docker run --rm python:3.11-alpine sh -c 'pip install -q bcrypt && python -c "import bcrypt; print(bcrypt.hashpw(b\"password\", bcrypt.gensalt(10)).decode())"'
> ```

- [ ] **Step 13: ビルド確認 + コミット**

```bash
cd 06_microservie/services/user-auth && go build -o /tmp/user-auth-bin .
cd /Users/yui/Documents/workspace/tanaka-yui/learning
git add 06_microservie/proto/user 06_microservie/services/user-auth 06_microservie/go.work
git commit -m "microservices(user-auth): add JWT-based auth service with bcrypt password hashing"
```

---

### Task 3: Payment サービスを追加（FLAKE_RATE 付き）

**Files:**
- Create: `06_microservie/proto/payment/v1/payment.proto`
- Create: `06_microservie/services/payment/{go.mod, Dockerfile, main.go, migrations/001_*, internal/{repo,server,obs,flake}/*}`
- Modify: `06_microservie/go.work`

- [ ] **Step 1: proto**

Create `06_microservie/proto/payment/v1/payment.proto`:

```proto
syntax = "proto3";

package payment.v1;

service PaymentService {
  rpc Charge(ChargeRequest) returns (ChargeResponse);
  rpc Refund(RefundRequest) returns (RefundResponse);
}

message ChargeRequest {
  string idempotency_key = 1;
  string order_id = 2;
  int32 amount_cents = 3;
}

message ChargeResponse {
  string payment_id = 1;
  string status = 2; // "succeeded"
}

message RefundRequest {
  string payment_id = 1;
}

message RefundResponse {
  string status = 1; // "refunded"
}
```

- [ ] **Step 2: コード生成**

`buf generate`

- [ ] **Step 3: マイグレーション**

Create `06_microservie/services/payment/migrations/001_create_payments.sql`:

```sql
CREATE TABLE IF NOT EXISTS payments (
    id              TEXT PRIMARY KEY,
    idempotency_key TEXT UNIQUE NOT NULL,
    order_id        TEXT NOT NULL,
    amount_cents    INTEGER NOT NULL CHECK (amount_cents > 0),
    status          TEXT NOT NULL CHECK (status IN ('succeeded','refunded')),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

- [ ] **Step 4: go.mod**

```
module microservie/payment

go 1.25

require (
    microservie/proto v0.0.0
    github.com/jackc/pgx/v5 v5.7.1
    github.com/google/uuid v1.6.0
    google.golang.org/grpc v1.66.0
)

replace microservie/proto => ../../proto
```

go.work に `./services/payment` を追加。

- [ ] **Step 5: flake パッケージのTDD**

Create `06_microservie/services/payment/internal/flake/flake_test.go`:

```go
package flake_test

import (
	"testing"

	"microservie/payment/internal/flake"
)

func TestShouldFail_zeroRateNeverFails(t *testing.T) {
	f := flake.New(0.0, 42)
	for i := 0; i < 100; i++ {
		if f.ShouldFail() {
			t.Fatalf("rate=0 returned true at iter %d", i)
		}
	}
}

func TestShouldFail_fullRateAlwaysFails(t *testing.T) {
	f := flake.New(1.0, 42)
	for i := 0; i < 100; i++ {
		if !f.ShouldFail() {
			t.Fatalf("rate=1 returned false at iter %d", i)
		}
	}
}

func TestShouldFail_approximateRate(t *testing.T) {
	f := flake.New(0.3, 42) // 同じ seed で再現性
	fails := 0
	const n = 10000
	for i := 0; i < n; i++ {
		if f.ShouldFail() {
			fails++
		}
	}
	rate := float64(fails) / float64(n)
	if rate < 0.27 || rate > 0.33 {
		t.Fatalf("want ~0.30, got %.3f", rate)
	}
}
```

Create `06_microservie/services/payment/internal/flake/flake.go`:

```go
package flake

import (
	"math/rand"
	"sync"
)

type Flake struct {
	rate float64
	rng  *rand.Rand
	mu   sync.Mutex
}

func New(rate float64, seed int64) *Flake {
	return &Flake{rate: rate, rng: rand.New(rand.NewSource(seed))}
}

func (f *Flake) ShouldFail() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.rng.Float64() < f.rate
}
```

Run: `go test ./internal/flake/...`. Expect: 3 テストパス。

- [ ] **Step 6: repo のTDD**

Same pattern as inventory/users repo. Files:
- `internal/repo/payments.go`: `Create(ctx, idempotencyKey, orderID, amount, status) (id, error)`, `GetByIdempotencyKey(ctx, key) (Payment, error)`, `MarkRefunded(ctx, id) error`
- `internal/repo/payments_test.go`: 3 tests covering insert, lookup-by-idempotency-key (returns existing for duplicate), and refund.

実装は省略（catalog/products.go を雛形に同等で書く）。

- [ ] **Step 7: server 実装**

Create `06_microservie/services/payment/internal/server/grpc.go`:

```go
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
```

Test: bufconn パターンで `TestCharge_idempotentReplay` と `TestCharge_failsWhenFlaked` の2テスト（fakeRepo + Flake(1.0,42)）。

- [ ] **Step 8: main.go**

Pattern identical to inventory/main.go. Differences:
- service name = `payment`
- default port = `50055`
- FLAKE_RATE env var: `flake.New(rateFromEnv("FLAKE_RATE", 0.0), time.Now().UnixNano())`

```go
package main

// ... imports as inventory/main.go pattern + microservie/payment/internal/flake

func main() {
    // ... same observability + DB setup
    rate, _ := strconv.ParseFloat(os.Getenv("FLAKE_RATE"), 64)
    fl := flake.New(rate, time.Now().UnixNano())
    paymentv1.RegisterPaymentServiceServer(gs, server.New(repo.New(pool), fl))
    // ...
}
```

- [ ] **Step 9: Dockerfile**

Pattern identical to inventory/Dockerfile with `payment` substitutions, port `50055`.

- [ ] **Step 10: ビルド + コミット**

```bash
cd 06_microservie/services/payment && go build -o /tmp/payment-bin .
cd /Users/yui/Documents/workspace/tanaka-yui/learning
git add 06_microservie/proto/payment 06_microservie/services/payment 06_microservie/go.work
git commit -m "microservices(payment): add Charge/Refund service with FLAKE_RATE simulation"
```

---

### Task 4: Order サービス（スケルトン、Saga はTask 5）

**Files:**
- Create: `06_microservie/proto/order/v1/order.proto`
- Create: `06_microservie/services/order/{go.mod, Dockerfile, main.go, migrations/001_*, internal/{repo,server,obs}/*}`
- Modify: `06_microservie/go.work`

- [ ] **Step 1: proto**

Create `06_microservie/proto/order/v1/order.proto`:

```proto
syntax = "proto3";

package order.v1;

import "inventory/v1/inventory.proto";

service OrderService {
  rpc PlaceOrder(PlaceOrderRequest) returns (PlaceOrderResponse);
  rpc GetOrder(GetOrderRequest) returns (GetOrderResponse);
  rpc ListOrders(ListOrdersRequest) returns (ListOrdersResponse);
}

message PlaceOrderRequest {
  string user_id = 1;
  repeated inventory.v1.Item items = 2;
}

message PlaceOrderResponse {
  string order_id = 1;
  string status = 2;  // CONFIRMED / FAILED
}

message Order {
  string id = 1;
  string user_id = 2;
  string status = 3;
  int32 total_cents = 4;
  repeated OrderItem items = 5;
}

message OrderItem {
  string product_id = 1;
  int32 quantity = 2;
  int32 unit_price_cents = 3;
}

message GetOrderRequest {
  string id = 1;
  string user_id = 2;
}

message GetOrderResponse {
  Order order = 1;
}

message ListOrdersRequest {
  string user_id = 1;
}

message ListOrdersResponse {
  repeated Order orders = 1;
}
```

- [ ] **Step 2: コード生成 + マイグレーション**

`buf generate`

Create `06_microservie/services/order/migrations/001_create_orders.sql`:

```sql
CREATE TABLE IF NOT EXISTS orders (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL,
    status      TEXT NOT NULL CHECK (status IN ('PENDING','CONFIRMED','FAILED')),
    total_cents INTEGER NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS order_items (
    order_id          TEXT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    product_id        TEXT NOT NULL,
    quantity          INTEGER NOT NULL CHECK (quantity > 0),
    unit_price_cents  INTEGER NOT NULL CHECK (unit_price_cents >= 0),
    PRIMARY KEY (order_id, product_id)
);

CREATE TABLE IF NOT EXISTS saga_log (
    order_id  TEXT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    step      TEXT NOT NULL,
    status    TEXT NOT NULL,
    detail    TEXT,
    at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (order_id, step)
);
```

- [ ] **Step 3: go.mod + go.work 追加**

```
module microservie/order

go 1.25

require (
    microservie/proto v0.0.0
    github.com/jackc/pgx/v5 v5.7.1
    github.com/google/uuid v1.6.0
    github.com/cenkalti/backoff/v4 v4.3.0
    github.com/sony/gobreaker v1.0.0
    google.golang.org/grpc v1.66.0
)

replace microservie/proto => ../../proto
```

- [ ] **Step 4: repo の最小実装（Saga は Task 5）**

Create `06_microservie/services/order/internal/repo/orders.go` with:
- `Create(ctx, userID, items) (orderID, error)` — orders + order_items を1トランザクションで挿入、status=PENDING
- `UpdateStatus(ctx, orderID, status) error`
- `Get(ctx, orderID, userID) (Order, error)` — user_id によるアクセス制御
- `List(ctx, userID) ([]Order, error)`
- `LogStep(ctx, orderID, step, status, detail) error` — saga_log への append（冪等: ON CONFLICT (order_id, step) DO UPDATE）

実装は inventory/users と同じパターン。テストは 4本:
- Create_thenGet
- UpdateStatus
- Get_otherUserCantSee
- LogStep_idempotent

- [ ] **Step 5: server スケルトン**

`internal/server/grpc.go`:

```go
package server

import (
	"context"
	// 詳細は Task 5 で Saga を呼ぶように差し替える
)

type Server struct {
	// orderRepo, sagaRunner (後で注入)
}

func New(/*deps*/) *Server { return &Server{} }

// PlaceOrder は Task 5 で本実装する。今は status=PENDING で返すだけ。
// GetOrder / ListOrders は repo に委譲（最小実装）。
```

Task 4 では PlaceOrder は ペイロードを単に注文として記録し PENDING を返すだけ。Saga は Task 5。

- [ ] **Step 6: obs + main + Dockerfile**

Same pattern. Default port 50053. service name `order`.

- [ ] **Step 7: ビルド + コミット**

```bash
git add 06_microservie/proto/order 06_microservie/services/order 06_microservie/go.work
git commit -m "microservices(order): add order service skeleton (Saga in next task)"
```

---

### Task 5: Saga 実装（order 内、checkout フロー）

**Files:**
- Create: `06_microservie/services/order/internal/saga/checkout.go`
- Create: `06_microservie/services/order/internal/saga/checkout_test.go`
- Modify: `06_microservie/services/order/internal/server/grpc.go`
- Modify: `06_microservie/services/order/main.go` (依存性注入)

- [ ] **Step 1: Saga の TDD - テストを書く**

Create `06_microservie/services/order/internal/saga/checkout_test.go`:

```go
package saga_test

import (
	"context"
	"errors"
	"testing"

	"microservie/order/internal/saga"
)

type fakeInv struct {
	reserveErr error
	commitErr  error
	releaseErr error
	calls      []string
}

func (f *fakeInv) Reserve(ctx context.Context, orderID string, items []saga.Item) (string, error) {
	f.calls = append(f.calls, "Reserve")
	return "r-1", f.reserveErr
}
func (f *fakeInv) Commit(ctx context.Context, resID string) error {
	f.calls = append(f.calls, "Commit")
	return f.commitErr
}
func (f *fakeInv) Release(ctx context.Context, resID string) error {
	f.calls = append(f.calls, "Release")
	return f.releaseErr
}

type fakePay struct {
	chargeErr error
	refundErr error
	calls     []string
}

func (f *fakePay) Charge(ctx context.Context, idem, orderID string, amount int32) (string, error) {
	f.calls = append(f.calls, "Charge")
	return "p-1", f.chargeErr
}
func (f *fakePay) Refund(ctx context.Context, paymentID string) error {
	f.calls = append(f.calls, "Refund")
	return f.refundErr
}

type fakeOrder struct {
	statusUpdates []string
	steps         []string
}

func (f *fakeOrder) UpdateStatus(ctx context.Context, orderID, st string) error {
	f.statusUpdates = append(f.statusUpdates, st)
	return nil
}
func (f *fakeOrder) LogStep(ctx context.Context, orderID, step, st, detail string) error {
	f.steps = append(f.steps, step+":"+st)
	return nil
}

func TestCheckout_happyPath(t *testing.T) {
	inv := &fakeInv{}
	pay := &fakePay{}
	ord := &fakeOrder{}
	c := saga.NewCheckout(inv, pay, ord)

	err := c.Run(context.Background(), saga.Input{
		OrderID: "o-1", Items: []saga.Item{{ProductID: "p-001", Quantity: 1}}, TotalCents: 480,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	want := []string{"Reserve", "Charge", "Commit"}
	if !equal(inv.calls, []string{"Reserve", "Commit"}) || !equal(pay.calls, []string{"Charge"}) {
		t.Fatalf("call sequence wrong inv=%v pay=%v want subseq %v", inv.calls, pay.calls, want)
	}
	if last(ord.statusUpdates) != "CONFIRMED" {
		t.Fatalf("want last status CONFIRMED, got %v", ord.statusUpdates)
	}
}

func TestCheckout_inventoryReserveFails_returnsError_noCompensation(t *testing.T) {
	inv := &fakeInv{reserveErr: errors.New("out of stock")}
	pay := &fakePay{}
	ord := &fakeOrder{}
	c := saga.NewCheckout(inv, pay, ord)

	err := c.Run(context.Background(), saga.Input{OrderID: "o-1"})
	if err == nil {
		t.Fatal("want error")
	}
	if len(pay.calls) != 0 {
		t.Fatalf("payment must not be called, got %v", pay.calls)
	}
	if last(ord.statusUpdates) != "FAILED" {
		t.Fatalf("want FAILED, got %v", ord.statusUpdates)
	}
}

func TestCheckout_paymentChargeFails_releasesReservation(t *testing.T) {
	inv := &fakeInv{}
	pay := &fakePay{chargeErr: errors.New("payment failed")}
	ord := &fakeOrder{}
	c := saga.NewCheckout(inv, pay, ord)

	err := c.Run(context.Background(), saga.Input{OrderID: "o-1"})
	if err == nil {
		t.Fatal("want error")
	}
	if !contains(inv.calls, "Release") {
		t.Fatalf("compensation Release missing: %v", inv.calls)
	}
	if last(ord.statusUpdates) != "FAILED" {
		t.Fatalf("want FAILED, got %v", ord.statusUpdates)
	}
}

func TestCheckout_inventoryCommitFails_refundsPayment(t *testing.T) {
	inv := &fakeInv{commitErr: errors.New("commit failed")}
	pay := &fakePay{}
	ord := &fakeOrder{}
	c := saga.NewCheckout(inv, pay, ord)

	err := c.Run(context.Background(), saga.Input{OrderID: "o-1"})
	if err == nil {
		t.Fatal("want error")
	}
	if !contains(pay.calls, "Refund") {
		t.Fatalf("compensation Refund missing: %v", pay.calls)
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
func contains(xs []string, t string) bool {
	for _, x := range xs {
		if x == t {
			return true
		}
	}
	return false
}
func last(xs []string) string {
	if len(xs) == 0 {
		return ""
	}
	return xs[len(xs)-1]
}
```

Run: expect compile errors.

- [ ] **Step 2: Saga 実装**

Create `06_microservie/services/order/internal/saga/checkout.go`:

```go
package saga

import (
	"context"
	"errors"
	"fmt"
)

type Item struct {
	ProductID string
	Quantity  int32
}

type Input struct {
	OrderID    string
	Items      []Item
	TotalCents int32
}

type Inventory interface {
	Reserve(ctx context.Context, orderID string, items []Item) (string, error)
	Commit(ctx context.Context, reservationID string) error
	Release(ctx context.Context, reservationID string) error
}

type Payment interface {
	Charge(ctx context.Context, idem, orderID string, amount int32) (string, error)
	Refund(ctx context.Context, paymentID string) error
}

type OrderStore interface {
	UpdateStatus(ctx context.Context, orderID, status string) error
	LogStep(ctx context.Context, orderID, step, status, detail string) error
}

type Checkout struct {
	inv Inventory
	pay Payment
	ord OrderStore
}

func NewCheckout(inv Inventory, pay Payment, ord OrderStore) *Checkout {
	return &Checkout{inv: inv, pay: pay, ord: ord}
}

func (c *Checkout) Run(ctx context.Context, in Input) error {
	// step1: Reserve
	resID, err := c.inv.Reserve(ctx, in.OrderID, in.Items)
	if err != nil {
		_ = c.ord.LogStep(ctx, in.OrderID, "reserve", "failed", err.Error())
		_ = c.ord.UpdateStatus(ctx, in.OrderID, "FAILED")
		return fmt.Errorf("reserve: %w", err)
	}
	_ = c.ord.LogStep(ctx, in.OrderID, "reserve", "ok", resID)

	// step2: Charge
	payID, err := c.pay.Charge(ctx, "pay-"+in.OrderID, in.OrderID, in.TotalCents)
	if err != nil {
		_ = c.ord.LogStep(ctx, in.OrderID, "charge", "failed", err.Error())
		// 補償: Reserve を取り消す
		if relErr := c.inv.Release(ctx, resID); relErr != nil {
			_ = c.ord.LogStep(ctx, in.OrderID, "release", "failed", relErr.Error())
		} else {
			_ = c.ord.LogStep(ctx, in.OrderID, "release", "ok", "")
		}
		_ = c.ord.UpdateStatus(ctx, in.OrderID, "FAILED")
		return fmt.Errorf("charge: %w", err)
	}
	_ = c.ord.LogStep(ctx, in.OrderID, "charge", "ok", payID)

	// step3: Commit
	if err := c.inv.Commit(ctx, resID); err != nil {
		_ = c.ord.LogStep(ctx, in.OrderID, "commit", "failed", err.Error())
		// 補償: Charge を Refund
		if refErr := c.pay.Refund(ctx, payID); refErr != nil {
			_ = c.ord.LogStep(ctx, in.OrderID, "refund", "failed", refErr.Error())
		} else {
			_ = c.ord.LogStep(ctx, in.OrderID, "refund", "ok", "")
		}
		_ = c.ord.UpdateStatus(ctx, in.OrderID, "FAILED")
		return fmt.Errorf("commit: %w", err)
	}
	_ = c.ord.LogStep(ctx, in.OrderID, "commit", "ok", "")
	_ = c.ord.UpdateStatus(ctx, in.OrderID, "CONFIRMED")
	return nil
}

// 静的型チェック
var _ Inventory = (interface {
	Reserve(ctx context.Context, orderID string, items []Item) (string, error)
	Commit(context.Context, string) error
	Release(context.Context, string) error
})(nil)

var _ = errors.New
```

> 注: 末尾の `var _ = errors.New` は import 未使用を抑制するためのダミー。実装側で `errors` を使うか、import を削るか調整。

- [ ] **Step 3: テストパス確認**

```bash
cd 06_microservie/services/order && go mod tidy && go test ./internal/saga/...
```

Expected: 4 テストパス

- [ ] **Step 4: server を Saga に接続**

Modify `06_microservie/services/order/internal/server/grpc.go` で `PlaceOrder` を:

```go
func (s *Server) PlaceOrder(ctx context.Context, req *orderv1.PlaceOrderRequest) (*orderv1.PlaceOrderResponse, error) {
	// catalog から単価を取得（or BFF が既に渡してくる設計にする）
	// 本実装では Items に unit_price_cents を含めて BFF から受け取る前提に修正することも可能だが、
	// 教材の単純化として: orderRepo.Create(userID, items) は items に単価を含んだ形を受け取り、
	// total を計算済みとして渡す。実装者は server で catalog gRPC を呼んで unit_price を埋める。
	// ↓実装の流れ
	orderID, total, err := s.orderRepo.Create(ctx, req.UserId, req.Items /* with prices */)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if err := s.saga.Run(ctx, saga.Input{OrderID: orderID, Items: toSagaItems(req.Items), TotalCents: total}); err != nil {
		return &orderv1.PlaceOrderResponse{OrderId: orderID, Status: "FAILED"}, nil
	}
	return &orderv1.PlaceOrderResponse{OrderId: orderID, Status: "CONFIRMED"}, nil
}
```

実装ノート: 簡略化のため、proto の `inventory.v1.Item` には `unit_price_cents` がない。Task 5 では `PlaceOrder` で受け取る前にカタログから単価を取得するか、もしくは proto に `unit_price_cents` を加えるか決める。後者を採用し、Task 5 の冒頭で proto に追記する案を取る:

Modify `06_microservie/proto/order/v1/order.proto`:

```proto
message PlaceOrderItem {
  string product_id = 1;
  int32 quantity = 2;
  int32 unit_price_cents = 3;
}

message PlaceOrderRequest {
  string user_id = 1;
  repeated PlaceOrderItem items = 2;
}
```

そして再生成（`buf generate`）。BFF 側で「カタログから商品 → 単価」を解決して PlaceOrder に渡すことになる（Task 7-8）。

- [ ] **Step 5: main.go で依存性注入**

main.go で:
- inventory gRPC クライアントをダイヤル → `sagaInv` を実装
- payment gRPC クライアントをダイヤル → `sagaPay` を実装
- saga.NewCheckout(sagaInv, sagaPay, orderRepo) を server.New に渡す

`internal/client/inventory.go`, `internal/client/payment.go` を作成し、それぞれ Saga インターフェースを実装。

- [ ] **Step 6: ビルド + コミット**

```bash
git add 06_microservie/services/order 06_microservie/proto/order
git commit -m "microservices(order): implement Saga checkout with compensating transactions"
```

---

### Task 6: レジリエンス（timeout / retry / circuit breaker）

**Files:**
- Create: `06_microservie/services/order/internal/resilience/breaker.go`
- Modify: `06_microservie/services/order/internal/client/payment.go` （CB ラップ）
- Modify: `06_microservie/services/order/internal/client/inventory.go` （retry ラップ）

- [ ] **Step 1: Circuit Breaker のラッパー**

Create `06_microservie/services/order/internal/resilience/breaker.go`:

```go
package resilience

import (
	"time"

	"github.com/sony/gobreaker"
)

func NewBreaker(name string) *gobreaker.CircuitBreaker {
	return gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        name,
		MaxRequests: 1,                // 半開時の試行数
		Interval:    10 * time.Second, // カウンタリセット間隔
		Timeout:     30 * time.Second, // Open 状態の持続時間
		ReadyToTrip: func(c gobreaker.Counts) bool {
			return c.Requests >= 5 && float64(c.TotalFailures)/float64(c.Requests) >= 0.5
		},
	})
}
```

テスト `internal/resilience/breaker_test.go` で以下を確認:
- 失敗を 5 連続で食わせると Open になる（`breaker.State() == gobreaker.StateOpen`）
- Open 状態では即座にエラーを返す（依存先を呼ばない）

- [ ] **Step 2: payment client に CB をラップ**

Modify `06_microservie/services/order/internal/client/payment.go`:

```go
type Payment struct {
	c       paymentv1.PaymentServiceClient
	breaker *gobreaker.CircuitBreaker
}

func DialPayment(addr string) (*Payment, error) {
	conn, err := grpc.NewClient(addr, ...)
	if err != nil {
		return nil, err
	}
	return &Payment{
		c:       paymentv1.NewPaymentServiceClient(conn),
		breaker: resilience.NewBreaker("payment.Charge"),
	}, nil
}

func (p *Payment) Charge(ctx context.Context, idem, orderID string, amount int32) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	res, err := p.breaker.Execute(func() (interface{}, error) {
		return p.c.Charge(ctx, &paymentv1.ChargeRequest{
			IdempotencyKey: idem, OrderId: orderID, AmountCents: amount,
		})
	})
	if err != nil {
		return "", err
	}
	r := res.(*paymentv1.ChargeResponse)
	return r.PaymentId, nil
}
```

- [ ] **Step 3: inventory client に retry をラップ**

Modify `06_microservie/services/order/internal/client/inventory.go`:

```go
import (
	"github.com/cenkalti/backoff/v4"
)

func (i *Inventory) Reserve(ctx context.Context, orderID string, items []saga.Item) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	var resID string
	op := func() error {
		// Reserve は冪等ではないため、ネットワーク失敗のみ再試行可
		// codes.Unavailable / codes.DeadlineExceeded のみ retry とし、
		// FailedPrecondition（在庫不足）は backoff.Permanent でリトライ停止
		r, err := i.c.Reserve(ctx, ...)
		if err != nil {
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
```

> 注: `Reserve` はサーバ側で冪等ではないので、リクエスト ID で重複検知できる場合のみリトライすべき。教材では「リトライしすぎると二重予約のリスクがある」点をコメントで明示し、最大3回に絞る。

- [ ] **Step 4: ビルド + コミット**

```bash
git add 06_microservie/services/order
git commit -m "microservices(order): add timeout/retry to inventory, circuit breaker to payment"
```

---

### Task 7: BFF 認証ミドルウェア + auth エンドポイント

**Files:**
- Create: `06_microservie/bff/internal/middleware/auth.go`
- Create: `06_microservie/bff/internal/handler/auth.go`
- Create: `06_microservie/bff/internal/client/user_auth.go`
- Modify: `06_microservie/bff/main.go`

- [ ] **Step 1: user-auth gRPC クライアント**

Create `06_microservie/bff/internal/client/user_auth.go`:

```go
package client

import (
	"context"

	userv1 "microservie/proto/gen/go/user/v1"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type UserAuth struct{ c userv1.UserServiceClient }

func DialUserAuth(addr string) (*UserAuth, error) {
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
	if err != nil {
		return nil, err
	}
	return &UserAuth{c: userv1.NewUserServiceClient(conn)}, nil
}

func (u *UserAuth) SignUp(ctx context.Context, email, password string) (string, error) {
	r, err := u.c.SignUp(ctx, &userv1.SignUpRequest{Email: email, Password: password})
	if err != nil {
		return "", err
	}
	return r.UserId, nil
}

func (u *UserAuth) SignIn(ctx context.Context, email, password string) (string, error) {
	r, err := u.c.SignIn(ctx, &userv1.SignInRequest{Email: email, Password: password})
	if err != nil {
		return "", err
	}
	return r.Token, nil
}

func (u *UserAuth) ValidateToken(ctx context.Context, token string) (string, error) {
	r, err := u.c.ValidateToken(ctx, &userv1.ValidateTokenRequest{Token: token})
	if err != nil {
		return "", err
	}
	return r.UserId, nil
}
```

- [ ] **Step 2: 認証ミドルウェア**

Create `06_microservie/bff/internal/middleware/auth.go`:

```go
package middleware

import (
	"context"
	"net/http"
)

type ctxKey string

const userIDKey ctxKey = "user_id"

type Validator interface {
	ValidateToken(ctx context.Context, token string) (string, error)
}

func Auth(v Validator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c, err := r.Cookie("session")
			if err != nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			uid, err := v.ValidateToken(r.Context(), c.Value)
			if err != nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), userIDKey, uid)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func UserID(ctx context.Context) string {
	v, _ := ctx.Value(userIDKey).(string)
	return v
}
```

テスト `middleware/auth_test.go`:
- Cookie 無し → 401
- 無効 Cookie → 401
- 有効 Cookie → ハンドラに UserID(ctx) で取れる

- [ ] **Step 3: auth ハンドラ**

Create `06_microservie/bff/internal/handler/auth.go`:

```go
package handler

import (
	"context"
	"encoding/json"
	"net/http"
)

type AuthClient interface {
	SignUp(ctx context.Context, email, password string) (string, error)
	SignIn(ctx context.Context, email, password string) (string, error)
}

type Auth struct{ c AuthClient }

func NewAuth(c AuthClient) *Auth { return &Auth{c: c} }

func (a *Auth) SignUp(w http.ResponseWriter, r *http.Request) {
	var req struct{ Email, Password string }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	uid, err := a.c.SignUp(r.Context(), req.Email, req.Password)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]string{"user_id": uid})
}

func (a *Auth) SignIn(w http.ResponseWriter, r *http.Request) {
	var req struct{ Email, Password string }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	token, err := a.c.SignIn(r.Context(), req.Email, req.Password)
	if err != nil {
		http.Error(w, "invalid credentials", 401)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	w.WriteHeader(http.StatusNoContent)
}
```

- [ ] **Step 4: main.go に統合**

`bff/main.go` を更新:
- DialUserAuth で client 作成
- `/api/auth/signup`, `/api/auth/signin` を Auth ハンドラに割当
- `r.Group(func(p chi.Router) { p.Use(middleware.Auth(uaClient)); /* 認証必須ルート */ })` で保護されたグループを定義（次タスクで /checkout などを追加）

- [ ] **Step 5: ビルド + コミット**

```bash
git add 06_microservie/bff
git commit -m "microservices(bff): add JWT auth middleware and signup/signin endpoints"
```

---

### Task 8: BFF checkout + orders エンドポイント

**Files:**
- Create: `06_microservie/bff/internal/client/order.go`
- Create: `06_microservie/bff/internal/handler/checkout.go`
- Create: `06_microservie/bff/internal/handler/orders.go`
- Modify: `06_microservie/bff/main.go`

- [ ] **Step 1: order gRPC クライアント**

`bff/internal/client/order.go` — DialOrder, PlaceOrder, GetOrder, ListOrders を実装。Catalog にも GetProduct を追加で取得して checkout 用の単価を埋める。

- [ ] **Step 2: checkout ハンドラ**

`bff/internal/handler/checkout.go`:

```go
func (h *Checkout) Post(w http.ResponseWriter, r *http.Request) {
	uid := middleware.UserID(r.Context())
	var req struct {
		Items []struct { ProductID string `json:"product_id"`; Quantity int32 `json:"quantity"` } `json:"items"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	// 各 product_id に対して catalog から単価を取得
	items := make([]*orderv1.PlaceOrderItem, 0, len(req.Items))
	for _, it := range req.Items {
		p, err := h.cat.GetProduct(r.Context(), it.ProductID)
		if err != nil {
			http.Error(w, "product lookup failed", 502)
			return
		}
		items = append(items, &orderv1.PlaceOrderItem{ProductId: it.ProductID, Quantity: it.Quantity, UnitPriceCents: p.PriceCents})
	}

	res, err := h.ord.PlaceOrder(r.Context(), uid, items)
	if err != nil {
		http.Error(w, err.Error(), 502)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"order_id": res.OrderId, "status": res.Status})
}
```

- [ ] **Step 3: orders ハンドラ**

`bff/internal/handler/orders.go` で `GET /api/orders` と `GET /api/orders/:id`。両方とも middleware.UserID(ctx) を渡して order.ListOrders / order.GetOrder。

- [ ] **Step 4: テスト + main 統合 + コミット**

各 handler に最低1つの単体テスト（fake client）を追加。main.go で保護グループに `/api/checkout`, `/api/orders`, `/api/orders/{id}` を割当。

```bash
git add 06_microservie/bff
git commit -m "microservices(bff): add /api/checkout and /api/orders endpoints"
```

---

### Task 9: docker-compose 拡張 + シード統合

**Files:**
- Modify: `06_microservie/docker-compose.yml`
- Modify: `06_microservie/Makefile`

- [ ] **Step 1: docker-compose に4 Postgres + 4 services を追加**

Replace `06_microservie/docker-compose.yml` with the complete configuration:

```yaml
services:
  # === Postgres インスタンス（DB-per-service の見える化） ===
  postgres-catalog:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: catalog
      POSTGRES_PASSWORD: catalog
      POSTGRES_DB: catalog
    ports: ["55432:5432"]
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U catalog -d catalog"]
      interval: 2s
      timeout: 2s
      retries: 20

  postgres-inventory:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: inventory
      POSTGRES_PASSWORD: inventory
      POSTGRES_DB: inventory
    ports: ["55433:5432"]
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U inventory -d inventory"]
      interval: 2s
      timeout: 2s
      retries: 20

  postgres-user-auth:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: userauth
      POSTGRES_PASSWORD: userauth
      POSTGRES_DB: userauth
    ports: ["55434:5432"]
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U userauth -d userauth"]
      interval: 2s
      timeout: 2s
      retries: 20

  postgres-payment:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: payment
      POSTGRES_PASSWORD: payment
      POSTGRES_DB: payment
    ports: ["55435:5432"]
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U payment -d payment"]
      interval: 2s
      timeout: 2s
      retries: 20

  postgres-order:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: orders
      POSTGRES_PASSWORD: orders
      POSTGRES_DB: orders
    ports: ["55436:5432"]
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U orders -d orders"]
      interval: 2s
      timeout: 2s
      retries: 20

  # === バックエンドサービス ===
  catalog:
    build: { context: ., dockerfile: services/catalog/Dockerfile }
    environment:
      DATABASE_URL: postgres://catalog:catalog@postgres-catalog:5432/catalog?sslmode=disable
      OTEL_EXPORTER_OTLP_ENDPOINT: otel-collector:4317
      GRPC_PORT: "50051"
    depends_on:
      postgres-catalog: { condition: service_healthy }
      otel-collector:   { condition: service_started }
    ports: ["50051:50051"]

  inventory:
    build: { context: ., dockerfile: services/inventory/Dockerfile }
    environment:
      DATABASE_URL: postgres://inventory:inventory@postgres-inventory:5432/inventory?sslmode=disable
      OTEL_EXPORTER_OTLP_ENDPOINT: otel-collector:4317
      GRPC_PORT: "50054"
    depends_on:
      postgres-inventory: { condition: service_healthy }
      otel-collector:     { condition: service_started }
    ports: ["50054:50054"]

  user-auth:
    build: { context: ., dockerfile: services/user-auth/Dockerfile }
    environment:
      DATABASE_URL: postgres://userauth:userauth@postgres-user-auth:5432/userauth?sslmode=disable
      OTEL_EXPORTER_OTLP_ENDPOINT: otel-collector:4317
      JWT_SECRET: test-secret-32-bytes-long-padding-pad
      GRPC_PORT: "50052"
    depends_on:
      postgres-user-auth: { condition: service_healthy }
      otel-collector:     { condition: service_started }
    ports: ["50052:50052"]

  payment:
    build: { context: ., dockerfile: services/payment/Dockerfile }
    environment:
      DATABASE_URL: postgres://payment:payment@postgres-payment:5432/payment?sslmode=disable
      OTEL_EXPORTER_OTLP_ENDPOINT: otel-collector:4317
      FLAKE_RATE: "${FLAKE_RATE:-0.0}"
      GRPC_PORT: "50055"
    depends_on:
      postgres-payment: { condition: service_healthy }
      otel-collector:   { condition: service_started }
    ports: ["50055:50055"]

  order:
    build: { context: ., dockerfile: services/order/Dockerfile }
    environment:
      DATABASE_URL: postgres://orders:orders@postgres-order:5432/orders?sslmode=disable
      OTEL_EXPORTER_OTLP_ENDPOINT: otel-collector:4317
      INVENTORY_ADDR: inventory:50054
      PAYMENT_ADDR: payment:50055
      CATALOG_ADDR: catalog:50051
      GRPC_PORT: "50053"
    depends_on:
      postgres-order: { condition: service_healthy }
      inventory:      { condition: service_started }
      payment:        { condition: service_started }
      otel-collector: { condition: service_started }
    ports: ["50053:50053"]

  bff:
    build: { context: ., dockerfile: bff/Dockerfile }
    environment:
      CATALOG_ADDR: catalog:50051
      USER_AUTH_ADDR: user-auth:50052
      ORDER_ADDR: order:50053
      OTEL_EXPORTER_OTLP_ENDPOINT: otel-collector:4317
      HTTP_PORT: "8080"
    depends_on:
      - catalog
      - user-auth
      - order
    ports: ["8080:8080"]

  # === 観測性スタック ===
  otel-collector:
    image: otel/opentelemetry-collector-contrib:0.110.0
    command: ["--config=/etc/otel/config.yaml"]
    volumes:
      - ./infra/otel-collector/config.yaml:/etc/otel/config.yaml:ro
    depends_on:
      - jaeger
    ports: ["4317:4317", "4318:4318"]

  jaeger:
    image: jaegertracing/all-in-one:1.76.0
    environment:
      COLLECTOR_OTLP_ENABLED: "true"
    ports: ["16686:16686", "4327:4317"]
```

> 注: `FLAKE_RATE: "${FLAKE_RATE:-0.0}"` で host 側 env var が `make up/flaky-XX` から渡される。デフォルトは 0.0（失敗しない）。

- [ ] **Step 2: Makefile に seed ターゲットを拡張**

```makefile
seed: seed/catalog seed/inventory seed/user-auth ## 初期データ投入

seed/catalog:
	docker compose exec -T postgres-catalog psql -U catalog -d catalog -f - < services/catalog/seed/seed.sql

seed/inventory:
	docker compose exec -T postgres-inventory psql -U inventory -d inventory -f - < services/inventory/seed/seed.sql

seed/user-auth:
	docker compose exec -T postgres-user-auth psql -U userauth -d userauth -f - < services/user-auth/seed/seed.sql
```

- [ ] **Step 3: 起動確認**

```bash
cd 06_microservie
make up
sleep 15
docker compose ps
```

Expected: 14 サービスが running または healthy（5 Postgres + 5 backends + bff + otel + jaeger）

- [ ] **Step 4: コミット**

```bash
git add 06_microservie/docker-compose.yml 06_microservie/Makefile
git commit -m "microservices: add inventory/user-auth/payment/order to docker-compose with seed targets"
```

---

### Task 10: Demo スクリプト + E2E 検証

**Files:**
- Modify: `06_microservie/Makefile`
- Modify: `06_microservie/VERIFICATION.md`

- [ ] **Step 1: demo:happy ターゲット**

Add to Makefile:

```makefile
demo/happy: ## 注文1件を成功させて trace を作る
	@SESSION=$$(curl -s -c - -X POST http://localhost:8080/api/auth/signin \
	  -H 'Content-Type: application/json' \
	  -d '{"email":"alice@example.com","password":"password"}' \
	  | grep session | awk '{print $$7}'); \
	curl -s -X POST http://localhost:8080/api/checkout \
	  -H "Cookie: session=$$SESSION" -H 'Content-Type: application/json' \
	  -d '{"items":[{"product_id":"p-001","quantity":1}]}' ; echo
```

- [ ] **Step 2: demo:retry**

```makefile
demo/retry: ## FLAKE_RATE=0.2 で複数回注文 → retry観察
	docker compose exec payment env FLAKE_RATE=0.2 /app/payment & sleep 1
	@for i in 1 2 3 4 5 6 7 8 9 10; do \
	  $(MAKE) demo/happy >/dev/null 2>&1 || true; \
	done
	@echo "Done. Inspect: docker compose logs order | grep -E 'retry|charge'"
```

> 実装ノート: FLAKE_RATE を起動後に変える簡単な仕組みは payment サービスが持っていない。代替案として、demo を「事前に FLAKE_RATE を env で指定して `make up` した状態を前提とする」設計にする。Makefile では `make up:flaky` のような起動ターゲットを別途用意:

```makefile
up/flaky-20: ## FLAKE_RATE=0.2 で起動
	FLAKE_RATE=0.2 docker compose up -d --build

up/flaky-60: ## FLAKE_RATE=0.6 で起動
	FLAKE_RATE=0.6 docker compose up -d --build
```

そして demo:retry / demo:circuit は単に複数回 `make demo:happy` を回すだけにする。

- [ ] **Step 3: demo:circuit**

```makefile
demo/circuit: ## demo:retry と同じだが FLAKE_RATE=0.6 で起動した状態を前提
	@for i in $$(seq 1 20); do \
	  $(MAKE) demo/happy >/dev/null 2>&1 || true; \
	done
	@docker compose logs order | grep -E 'breaker|CB|circuit' | tail -20
```

- [ ] **Step 4: E2E 検証**

Run sequence:
```bash
make up
sleep 15
make seed
make demo/happy
```

Verify:
- BFF が `{"order_id":"...", "status":"CONFIRMED"}` を返す
- Jaeger UI で trace を確認:
  ```bash
  sleep 3
  curl -s 'http://localhost:16686/api/traces?service=order&limit=1' | head -c 2000
  ```
  期待: `order → inventory.Reserve → payment.Charge → inventory.Commit` の span 階層が見える

Then test the retry path:
```bash
make down
make up/flaky-20
sleep 15
make seed
for i in $(seq 1 10); do make demo/happy; done
docker compose logs order | grep -E 'retry|charge.fail'
```

Expected: いくつかは payment 失敗で retry、最終的にほとんどが CONFIRMED

Then test the circuit breaker:
```bash
make down
make up/flaky-60
sleep 15
make seed
for i in $(seq 1 20); do make demo/happy; done
docker compose logs order | grep -E 'gobreaker|state=open|open'
```

Expected: `gobreaker: state=open` または同等のログが出る。一時的に payment.Charge への呼び出しが即時失敗する。

- [ ] **Step 5: VERIFICATION.md 更新**

Overwrite `06_microservie/VERIFICATION.md` with Plan 2 results:

```markdown
# Plan 2 (Services + Saga + Resilience) Verification Log

実施日: <YYYY-MM-DD>
ブランチ: feat/microservices

## 合格項目

- [x] `make up` で14コンテナが起動・healthy
- [x] `make seed` で 全初期データ投入
- [x] `make demo/happy` で注文確定（CONFIRMED）
- [x] Jaeger に `bff → order → inventory → payment → inventory.commit` の trace
- [x] `make up/flaky-20` + 連続注文で retry が発火し最終的に成功する注文が多い
- [x] `make up/flaky-60` + 連続注文で Circuit Breaker が Open に遷移
- [x] `make test` で全モジュールのテストパス

## 実行ログ抜粋

（curl/jaeger/log の実際の出力をここに貼る）

## Plan 3/4 への引き継ぎ

- React フロントエンド未実装 → Plan 3
- ドキュメント未執筆 → Plan 4
- pgx の OTel 計装（Postgres スパン）未実装
- slog の trace_id 自動注入未実装
```

- [ ] **Step 6: コミット**

```bash
git add 06_microservie/Makefile 06_microservie/VERIFICATION.md
git commit -m "microservices: add demo targets (happy/retry/circuit) and Plan 2 verification log"
```

---

## Plan 2 完了条件チェックリスト

- [ ] inventory / user-auth / payment / order の4サービスが Go モジュール+Postgres+gRPC+OTel+Dockerfile で揃っている
- [ ] `make up` で14コンテナが healthy
- [ ] `make seed` で全サービスの初期データが投入される
- [ ] `make demo/happy` でログイン→checkout→CONFIRMED まで通る
- [ ] Jaeger UI で checkout flow の trace が `bff → order → inventory → payment → inventory.commit` の階層で見える
- [ ] `make up/flaky-20` + 連続注文で retry の挙動が観察できる
- [ ] `make up/flaky-60` + 連続注文で Circuit Breaker の Open 遷移ログが出る
- [ ] `make test` で全モジュールのテストがパス

これらを満たしたら **Plan 3（React フロントエンド）** に進む準備が整う。
