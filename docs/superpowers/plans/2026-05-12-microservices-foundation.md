# 06_microservie Plan 1: Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 06_microservie 章の基盤を構築する。`catalog` サービス1本を Postgres から gRPC、BFF 経由の REST、OTel 経由の分散トレースまで縦断的に動かし、後続プランの足場を作る。

**Architecture:** Go 1.26 のマルチモジュール構成（`go.work` 管理）。`catalog` サービス（gRPC + Postgres + OTel）、`bff`（REST → gRPC 変換 + OTel）、OTel Collector + Jaeger を docker-compose で起動する。proto は `buf` で管理し、共有モジュール `proto/gen/go` 経由で各サービスにインポートさせる。

**Tech Stack:**
- Go 1.26（既存章 `01_process_thread/go/Dockerfile` を踏襲、distroless ベース）
- gRPC: `google.golang.org/grpc` + `github.com/bufbuild/buf` で `.proto` を管理
- HTTP: `github.com/go-chi/chi/v5`
- Postgres: `github.com/jackc/pgx/v5`
- Observability: `go.opentelemetry.io/otel` + `otelgrpc` 計装、`log/slog` 構造ログ
- Container: OTel Collector（`otel/opentelemetry-collector-contrib:0.110.0`）+ Jaeger（`jaegertracing/all-in-one:1.62`）+ `postgres:16-alpine`
- Test: `testing` + `github.com/testcontainers/testcontainers-go` で integration

**完了条件:**
1. `make up` で全コンテナが healthy になる
2. `curl http://localhost:8080/api/products` が JSON で10件返す
3. Jaeger UI（`http://localhost:16686`）で `bff → catalog → postgres` の trace が一本見える
4. `docker compose logs catalog | grep <trace_id>` で同じ ID がログにも出る
5. `make test` がパスする

**スコープ外（後続プランで扱う）:**
- inventory / order / payment / user-auth サービス → Plan 2
- Saga・Circuit Breaker・Retry → Plan 2
- React フロントエンド → Plan 3
- ドキュメント執筆（docs/01_concepts.md 〜）→ Plan 4

---

## ファイル構成

```
06_microservie/
├── .gitignore
├── README.md                          # 章入口（暫定。Plan 4 で本格執筆）
├── go.work                            # マルチモジュール workspace
├── buf.yaml
├── buf.gen.yaml
├── proto/
│   ├── go.mod                         # module microservie/proto
│   ├── catalog/v1/catalog.proto
│   └── gen/go/catalog/v1/             # buf generate で生成（.gitignore 除外）
├── services/
│   └── catalog/
│       ├── go.mod                     # module microservie/catalog
│       ├── Dockerfile
│       ├── main.go
│       ├── migrations/
│       │   └── 001_create_products.sql
│       ├── internal/
│       │   ├── repo/products.go
│       │   ├── repo/products_test.go
│       │   ├── server/grpc.go
│       │   ├── server/grpc_test.go
│       │   └── obs/otel.go
│       └── seed/seed.go
├── bff/
│   ├── go.mod                         # module microservie/bff
│   ├── Dockerfile
│   ├── main.go
│   ├── internal/
│   │   ├── handler/products.go
│   │   ├── handler/products_test.go
│   │   └── obs/otel.go
│   └── internal/client/catalog.go
├── infra/
│   └── otel-collector/
│       └── config.yaml
├── docker-compose.yml
└── Makefile
```

---

### Task 1: ディレクトリ構造の初期化と go.work

**Files:**
- Create: `06_microservie/.gitignore`
- Create: `06_microservie/README.md`
- Create: `06_microservie/go.work`

- [ ] **Step 1: ディレクトリツリーを作成**

Run:
```bash
cd /Users/yui/Documents/workspace/tanaka-yui/learning
mkdir -p 06_microservie/{proto/catalog/v1,services/catalog/{internal/{repo,server,obs},migrations,seed},bff/internal/{handler,client,obs},infra/otel-collector}
```

Expected: コマンドがエラーなく完了

- [ ] **Step 2: `.gitignore` を作成**

Create `06_microservie/.gitignore`:

```
# build artifacts
/services/*/bin/
/bff/bin/

# generated code
/proto/gen/

# env
.env
.env.local
```

- [ ] **Step 3: 暫定 README を作成**

Create `06_microservie/README.md`:

```markdown
# 06_microservie: マイクロサービス学習プロジェクト

ECサイトを題材に、小規模マイクロサービスの実装パターンを学ぶ章。

> 本章は段階的に構築中です。詳細なドキュメントは `docs/` 配下に追加されます。

## 起動

\`\`\`bash
make up      # 全サービス起動
make down    # 停止
make logs    # ログ
\`\`\`

## アクセス先

| URL | 用途 |
|---|---|
| http://localhost:8080/api/products | BFF REST API |
| http://localhost:16686 | Jaeger UI（分散トレース） |

詳細な学習動線は `docs/` に整備予定。
```

- [ ] **Step 4: `go.work` を作成**

Create `06_microservie/go.work`:

```
go 1.26

use (
    ./proto
    ./services/catalog
    ./bff
)
```

- [ ] **Step 5: コミット**

```bash
cd /Users/yui/Documents/workspace/tanaka-yui/learning
git add 06_microservie/
git commit -m "microservices: scaffold 06_microservie directory and go.work"
```

---

### Task 2: proto + buf セットアップと catalog.proto 生成

**Files:**
- Create: `06_microservie/buf.yaml`
- Create: `06_microservie/buf.gen.yaml`
- Create: `06_microservie/proto/go.mod`
- Create: `06_microservie/proto/catalog/v1/catalog.proto`

- [ ] **Step 1: buf 設定を作成**

Create `06_microservie/buf.yaml`:

```yaml
version: v2
modules:
  - path: proto
lint:
  use:
    - STANDARD
breaking:
  use:
    - FILE
```

Create `06_microservie/buf.gen.yaml`:

```yaml
version: v2
managed:
  enabled: true
  override:
    - file_option: go_package_prefix
      value: microservie/proto/gen/go
plugins:
  - remote: buf.build/protocolbuffers/go
    out: proto/gen/go
    opt: paths=source_relative
  - remote: buf.build/grpc/go
    out: proto/gen/go
    opt:
      - paths=source_relative
      - require_unimplemented_servers=false
```

- [ ] **Step 2: `proto/go.mod` を作成**

Create `06_microservie/proto/go.mod`:

```
module microservie/proto

go 1.26

require (
    google.golang.org/grpc v1.66.0
    google.golang.org/protobuf v1.34.2
)
```

- [ ] **Step 3: catalog.proto を作成**

Create `06_microservie/proto/catalog/v1/catalog.proto`:

```proto
syntax = "proto3";

package catalog.v1;

service CatalogService {
  rpc ListProducts(ListProductsRequest) returns (ListProductsResponse);
  rpc GetProduct(GetProductRequest) returns (GetProductResponse);
}

message Product {
  string id = 1;
  string name = 2;
  string description = 3;
  int32 price_cents = 4;
}

message ListProductsRequest {
  int32 limit = 1;
  int32 offset = 2;
}

message ListProductsResponse {
  repeated Product products = 1;
}

message GetProductRequest {
  string id = 1;
}

message GetProductResponse {
  Product product = 1;
}
```

- [ ] **Step 4: buf でコード生成**

Run:
```bash
cd /Users/yui/Documents/workspace/tanaka-yui/learning/06_microservie
buf generate
```

Expected: `proto/gen/go/catalog/v1/catalog.pb.go` と `catalog_grpc.pb.go` が生成される。

- [ ] **Step 5: proto モジュールの依存を解決（go.sum 生成）**

Run:
```bash
cd /Users/yui/Documents/workspace/tanaka-yui/learning/06_microservie/proto
go mod tidy
```

Expected: `go.sum` が生成され、エラーなく終了

- [ ] **Step 6: 生成物の検証**

Run:
```bash
ls 06_microservie/proto/gen/go/catalog/v1/
ls 06_microservie/proto/go.sum
```

Expected: `catalog.pb.go` と `catalog_grpc.pb.go` の2ファイル + `go.sum` 存在

- [ ] **Step 7: コミット**

```bash
cd /Users/yui/Documents/workspace/tanaka-yui/learning
git add 06_microservie/buf.yaml 06_microservie/buf.gen.yaml 06_microservie/proto/
git commit -m "microservices: add catalog.proto and buf code generation"
```

---

### Task 3: Catalog サービス Go モジュール初期化

**Files:**
- Create: `06_microservie/services/catalog/go.mod`
- Create: `06_microservie/services/catalog/main.go`

- [ ] **Step 1: `go.mod` を作成**

Create `06_microservie/services/catalog/go.mod`:

```
module microservie/catalog

go 1.26

require (
    microservie/proto v0.0.0
    google.golang.org/grpc v1.66.0
    github.com/jackc/pgx/v5 v5.7.1
)

replace microservie/proto => ../../proto
```

- [ ] **Step 2: 最小の `main.go` を作成（gRPCサーバ stub）**

Create `06_microservie/services/catalog/main.go`:

```go
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"

	catalogv1 "microservie/proto/gen/go/catalog/v1"

	"google.golang.org/grpc"
)

type stubServer struct{}

func (s *stubServer) ListProducts(ctx context.Context, req *catalogv1.ListProductsRequest) (*catalogv1.ListProductsResponse, error) {
	return &catalogv1.ListProductsResponse{Products: []*catalogv1.Product{}}, nil
}

func (s *stubServer) GetProduct(ctx context.Context, req *catalogv1.GetProductRequest) (*catalogv1.GetProductResponse, error) {
	return &catalogv1.GetProductResponse{}, nil
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	port := "50051"
	if v := os.Getenv("GRPC_PORT"); v != "" {
		port = v
	}

	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		slog.Error("listen failed", "err", err)
		os.Exit(1)
	}

	s := grpc.NewServer()
	catalogv1.RegisterCatalogServiceServer(s, &stubServer{})

	slog.Info("catalog gRPC server starting", "port", port)
	if err := s.Serve(lis); err != nil {
		slog.Error("serve failed", "err", err)
		os.Exit(1)
	}
	fmt.Println("shutting down")
}
```

- [ ] **Step 3: `go mod tidy` で依存を解決**

Run:
```bash
cd /Users/yui/Documents/workspace/tanaka-yui/learning/06_microservie/services/catalog
go mod tidy
```

Expected: `go.sum` が生成され、エラーなく終了

- [ ] **Step 4: ビルドを通す**

Run:
```bash
cd /Users/yui/Documents/workspace/tanaka-yui/learning/06_microservie/services/catalog
go build -o /tmp/catalog-bin ./...
```

Expected: エラーなくバイナリが `/tmp/catalog-bin` に生成

- [ ] **Step 5: コミット**

```bash
cd /Users/yui/Documents/workspace/tanaka-yui/learning
git add 06_microservie/services/catalog/
git commit -m "microservices: scaffold catalog gRPC server skeleton"
```

---

### Task 4: Catalog Postgres リポジトリ（TDD with testcontainers）

**Files:**
- Create: `06_microservie/services/catalog/migrations/001_create_products.sql`
- Create: `06_microservie/services/catalog/internal/repo/products.go`
- Create: `06_microservie/services/catalog/internal/repo/products_test.go`

- [ ] **Step 1: マイグレーション SQL を作成**

Create `06_microservie/services/catalog/migrations/001_create_products.sql`:

```sql
CREATE TABLE IF NOT EXISTS products (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    price_cents INTEGER NOT NULL CHECK (price_cents >= 0),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

- [ ] **Step 2: 失敗するテストを書く**

Create `06_microservie/services/catalog/internal/repo/products_test.go`:

```go
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
		testcontainers.WithWaitStrategy(wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(30*time.Second)),
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
```

- [ ] **Step 3: テストを実行して失敗を確認**

Run:
```bash
cd /Users/yui/Documents/workspace/tanaka-yui/learning/06_microservie/services/catalog
go mod tidy
go test ./internal/repo/...
```

Expected: コンパイルエラー（`repo.New`, `repo.Product`, `repo.ErrNotFound` 未定義）

- [ ] **Step 4: 最小実装**

Create `06_microservie/services/catalog/internal/repo/products.go`:

```go
package repo

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("product not found")

type Product struct {
	ID          string
	Name        string
	Description string
	PriceCents  int32
}

type Repo struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repo {
	return &Repo{pool: pool}
}

func (r *Repo) List(ctx context.Context, limit, offset int32) ([]Product, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, name, description, price_cents FROM products ORDER BY id LIMIT $1 OFFSET $2`,
		limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ps []Product
	for rows.Next() {
		var p Product
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.PriceCents); err != nil {
			return nil, err
		}
		ps = append(ps, p)
	}
	return ps, rows.Err()
}

func (r *Repo) Get(ctx context.Context, id string) (Product, error) {
	var p Product
	err := r.pool.QueryRow(ctx,
		`SELECT id, name, description, price_cents FROM products WHERE id = $1`,
		id,
	).Scan(&p.ID, &p.Name, &p.Description, &p.PriceCents)
	if errors.Is(err, pgx.ErrNoRows) {
		return Product{}, ErrNotFound
	}
	return p, err
}

func (r *Repo) Insert(ctx context.Context, p Product) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO products(id, name, description, price_cents) VALUES ($1,$2,$3,$4)`,
		p.ID, p.Name, p.Description, p.PriceCents,
	)
	return err
}
```

- [ ] **Step 5: テストがパスすることを確認**

Run:
```bash
cd /Users/yui/Documents/workspace/tanaka-yui/learning/06_microservie/services/catalog
go mod tidy
go test ./internal/repo/...
```

Expected: 3テスト全パス（`ok  microservie/catalog/internal/repo`）

> 注: testcontainers は Docker が必要。失敗する場合は Docker Desktop が起動しているか確認。

- [ ] **Step 6: コミット**

```bash
cd /Users/yui/Documents/workspace/tanaka-yui/learning
git add 06_microservie/services/catalog/
git commit -m "microservices(catalog): add products repo with testcontainers integration tests"
```

---

### Task 5: Catalog gRPC ハンドラを repo に接続

**Files:**
- Modify: `06_microservie/services/catalog/main.go`
- Create: `06_microservie/services/catalog/internal/server/grpc.go`
- Create: `06_microservie/services/catalog/internal/server/grpc_test.go`

- [ ] **Step 1: 失敗するテストを書く**

Create `06_microservie/services/catalog/internal/server/grpc_test.go`:

```go
package server_test

import (
	"context"
	"net"
	"testing"

	"microservie/catalog/internal/repo"
	"microservie/catalog/internal/server"
	catalogv1 "microservie/proto/gen/go/catalog/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

type fakeRepo struct{ products []repo.Product }

func (f *fakeRepo) List(ctx context.Context, limit, offset int32) ([]repo.Product, error) {
	return f.products, nil
}
func (f *fakeRepo) Get(ctx context.Context, id string) (repo.Product, error) {
	for _, p := range f.products {
		if p.ID == id {
			return p, nil
		}
	}
	return repo.Product{}, repo.ErrNotFound
}

func dial(t *testing.T, s *grpc.Server) catalogv1.CatalogServiceClient {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	go func() { _ = s.Serve(lis) }()
	t.Cleanup(s.Stop)

	conn, err := grpc.NewClient("passthrough://bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return catalogv1.NewCatalogServiceClient(conn)
}

func TestListProducts_returnsAll(t *testing.T) {
	fr := &fakeRepo{products: []repo.Product{
		{ID: "a", Name: "A", PriceCents: 100},
		{ID: "b", Name: "B", PriceCents: 200},
	}}
	gs := grpc.NewServer()
	catalogv1.RegisterCatalogServiceServer(gs, server.New(fr))

	client := dial(t, gs)
	res, err := client.ListProducts(context.Background(), &catalogv1.ListProductsRequest{Limit: 10})
	if err != nil {
		t.Fatalf("ListProducts: %v", err)
	}
	if len(res.Products) != 2 {
		t.Fatalf("want 2 products, got %d", len(res.Products))
	}
}

func TestGetProduct_notFoundReturnsError(t *testing.T) {
	fr := &fakeRepo{}
	gs := grpc.NewServer()
	catalogv1.RegisterCatalogServiceServer(gs, server.New(fr))

	client := dial(t, gs)
	_, err := client.GetProduct(context.Background(), &catalogv1.GetProductRequest{Id: "missing"})
	if err == nil {
		t.Fatal("want error, got nil")
	}
}
```

- [ ] **Step 2: テストを走らせて失敗確認**

Run:
```bash
cd /Users/yui/Documents/workspace/tanaka-yui/learning/06_microservie/services/catalog
go test ./internal/server/...
```

Expected: コンパイルエラー（`server.New` 未定義）

- [ ] **Step 3: server を実装**

Create `06_microservie/services/catalog/internal/server/grpc.go`:

```go
package server

import (
	"context"
	"errors"

	"microservie/catalog/internal/repo"
	catalogv1 "microservie/proto/gen/go/catalog/v1"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Reader interface {
	List(ctx context.Context, limit, offset int32) ([]repo.Product, error)
	Get(ctx context.Context, id string) (repo.Product, error)
}

type Server struct {
	r Reader
}

func New(r Reader) *Server {
	return &Server{r: r}
}

func (s *Server) ListProducts(ctx context.Context, req *catalogv1.ListProductsRequest) (*catalogv1.ListProductsResponse, error) {
	limit := req.Limit
	if limit <= 0 {
		limit = 50
	}
	ps, err := s.r.List(ctx, limit, req.Offset)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	out := make([]*catalogv1.Product, 0, len(ps))
	for _, p := range ps {
		out = append(out, toProto(p))
	}
	return &catalogv1.ListProductsResponse{Products: out}, nil
}

func (s *Server) GetProduct(ctx context.Context, req *catalogv1.GetProductRequest) (*catalogv1.GetProductResponse, error) {
	p, err := s.r.Get(ctx, req.Id)
	if errors.Is(err, repo.ErrNotFound) {
		return nil, status.Error(codes.NotFound, "product not found")
	}
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &catalogv1.GetProductResponse{Product: toProto(p)}, nil
}

func toProto(p repo.Product) *catalogv1.Product {
	return &catalogv1.Product{
		Id:          p.ID,
		Name:        p.Name,
		Description: p.Description,
		PriceCents:  p.PriceCents,
	}
}
```

- [ ] **Step 4: テストがパスすることを確認**

Run:
```bash
cd /Users/yui/Documents/workspace/tanaka-yui/learning/06_microservie/services/catalog
go mod tidy
go test ./internal/server/...
```

Expected: `ok  microservie/catalog/internal/server`

- [ ] **Step 5: `main.go` を repo + server 接続版に書き換え**

Replace `06_microservie/services/catalog/main.go` with:

```go
package main

import (
	"context"
	"log/slog"
	"net"
	"os"

	"microservie/catalog/internal/repo"
	"microservie/catalog/internal/server"
	catalogv1 "microservie/proto/gen/go/catalog/v1"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger.With("service", "catalog"))

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		slog.Error("DATABASE_URL is required")
		os.Exit(1)
	}

	ctx := context.Background()
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
		port = "50051"
	}
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		slog.Error("listen", "err", err)
		os.Exit(1)
	}

	gs := grpc.NewServer()
	catalogv1.RegisterCatalogServiceServer(gs, server.New(repo.New(pool)))

	slog.Info("catalog gRPC server starting", "port", port)
	if err := gs.Serve(lis); err != nil {
		slog.Error("serve", "err", err)
		os.Exit(1)
	}
}

func applyMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	b, err := os.ReadFile("/app/migrations/001_create_products.sql")
	if err != nil {
		// dev fallback to relative path
		b, err = os.ReadFile("migrations/001_create_products.sql")
		if err != nil {
			return err
		}
	}
	_, err = pool.Exec(ctx, string(b))
	return err
}
```

- [ ] **Step 6: ビルドして通ることを確認**

Run:
```bash
cd /Users/yui/Documents/workspace/tanaka-yui/learning/06_microservie/services/catalog
go build -o /tmp/catalog-bin ./...
```

Expected: エラーなくビルド完了

- [ ] **Step 7: コミット**

```bash
cd /Users/yui/Documents/workspace/tanaka-yui/learning
git add 06_microservie/services/catalog/
git commit -m "microservices(catalog): wire gRPC server to Postgres repo"
```

---

### Task 6: Catalog OTel 計装

**Files:**
- Create: `06_microservie/services/catalog/internal/obs/otel.go`
- Modify: `06_microservie/services/catalog/main.go`
- Modify: `06_microservie/services/catalog/go.mod`（依存追加）

- [ ] **Step 1: `obs/otel.go` を作成**

Create `06_microservie/services/catalog/internal/obs/otel.go`:

```go
package obs

import (
	"context"
	"log/slog"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

func InitTracing(ctx context.Context, serviceName string) (func(context.Context) error, error) {
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		endpoint = "otel-collector:4317"
	}

	exp, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(semconv.ServiceName(serviceName)),
	)
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	return tp.Shutdown, nil
}

// LogAttrsFromCtx は trace_id / span_id を slog 用の属性として返す。
func LogAttrsFromCtx(ctx context.Context) []slog.Attr {
	sc := trace.SpanContextFromContext(ctx)
	if !sc.IsValid() {
		return nil
	}
	return []slog.Attr{
		slog.String("trace_id", sc.TraceID().String()),
		slog.String("span_id", sc.SpanID().String()),
	}
}
```

- [ ] **Step 2: `main.go` を OTel + interceptor 対応に修正**

Replace `06_microservie/services/catalog/main.go` with:

```go
package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"microservie/catalog/internal/obs"
	"microservie/catalog/internal/repo"
	"microservie/catalog/internal/server"
	catalogv1 "microservie/proto/gen/go/catalog/v1"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger.With("service", "catalog"))

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	shutdownTracer, err := obs.InitTracing(ctx, "catalog")
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
		port = "50051"
	}
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		slog.Error("listen", "err", err)
		os.Exit(1)
	}

	gs := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	)
	catalogv1.RegisterCatalogServiceServer(gs, server.New(repo.New(pool)))

	go func() {
		<-ctx.Done()
		gs.GracefulStop()
	}()

	slog.Info("catalog gRPC server starting", "port", port)
	if err := gs.Serve(lis); err != nil {
		slog.Error("serve", "err", err)
		os.Exit(1)
	}
}

func applyMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	candidates := []string{"/app/migrations/001_create_products.sql", "migrations/001_create_products.sql"}
	var (
		b   []byte
		err error
	)
	for _, p := range candidates {
		b, err = os.ReadFile(p)
		if err == nil {
			break
		}
	}
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, string(b))
	return err
}
```

- [ ] **Step 3: 依存解決とビルド**

Run:
```bash
cd /Users/yui/Documents/workspace/tanaka-yui/learning/06_microservie/services/catalog
go mod tidy
go build -o /tmp/catalog-bin ./...
```

Expected: エラーなくビルド完了

- [ ] **Step 4: コミット**

```bash
cd /Users/yui/Documents/workspace/tanaka-yui/learning
git add 06_microservie/services/catalog/
git commit -m "microservices(catalog): add OpenTelemetry tracing and gRPC interceptor"
```

---

### Task 7: Catalog Dockerfile

**Files:**
- Create: `06_microservie/services/catalog/Dockerfile`

- [ ] **Step 1: Dockerfile を作成**

Create `06_microservie/services/catalog/Dockerfile`:

```dockerfile
FROM golang:1.26-bookworm AS builder

WORKDIR /work

# proto モジュールをコピー（go.work 経由の relative replace に対応）
COPY proto/go.mod proto/go.sum proto/
COPY proto/gen ./proto/gen

# catalog モジュール
WORKDIR /work/services/catalog
COPY services/catalog/go.mod services/catalog/go.sum ./
RUN go mod download

COPY services/catalog/ ./

RUN CGO_ENABLED=0 go build -o /out/catalog .

FROM gcr.io/distroless/static:nonroot
WORKDIR /app
COPY --from=builder /out/catalog /app/catalog
COPY services/catalog/migrations /app/migrations

ENV GRPC_PORT=50051
EXPOSE 50051

ENTRYPOINT ["/app/catalog"]
```

> 注: このイメージは `06_microservie/` をビルドコンテキストとしてビルドする想定。`docker-compose.yml` で `context: .` を指定する。

- [ ] **Step 2: コンテキストをルートにして単体ビルドを通す**

Run:
```bash
cd /Users/yui/Documents/workspace/tanaka-yui/learning/06_microservie
docker build -f services/catalog/Dockerfile -t catalog:dev .
```

Expected: イメージビルドが成功（`Successfully tagged catalog:dev`）

- [ ] **Step 3: コミット**

```bash
cd /Users/yui/Documents/workspace/tanaka-yui/learning
git add 06_microservie/services/catalog/Dockerfile
git commit -m "microservices(catalog): add multi-stage distroless Dockerfile"
```

---

### Task 8: BFF スケルトン（chi router + health）

**Files:**
- Create: `06_microservie/bff/go.mod`
- Create: `06_microservie/bff/main.go`
- Create: `06_microservie/bff/internal/handler/products.go`
- Create: `06_microservie/bff/internal/handler/products_test.go`

- [ ] **Step 1: `go.mod` を作成**

Create `06_microservie/bff/go.mod`:

```
module microservie/bff

go 1.26

require (
    microservie/proto v0.0.0
    github.com/go-chi/chi/v5 v5.1.0
    google.golang.org/grpc v1.66.0
)

replace microservie/proto => ../proto
```

- [ ] **Step 2: 失敗するハンドラテストを書く**

Create `06_microservie/bff/internal/handler/products_test.go`:

```go
package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"microservie/bff/internal/handler"
	catalogv1 "microservie/proto/gen/go/catalog/v1"
)

type fakeClient struct {
	products []*catalogv1.Product
}

func (f *fakeClient) ListProducts(ctx context.Context) ([]*catalogv1.Product, error) {
	return f.products, nil
}

func TestListProducts_returnsJSON(t *testing.T) {
	fc := &fakeClient{products: []*catalogv1.Product{
		{Id: "a", Name: "A", PriceCents: 100},
	}}
	h := handler.NewProducts(fc)

	req := httptest.NewRequest(http.MethodGet, "/api/products", nil)
	rec := httptest.NewRecorder()
	h.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}

	var body struct {
		Products []struct {
			ID         string `json:"id"`
			Name       string `json:"name"`
			PriceCents int32  `json:"price_cents"`
		} `json:"products"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Products) != 1 || body.Products[0].Name != "A" {
		t.Fatalf("unexpected body: %+v", body)
	}
}
```

- [ ] **Step 3: テスト失敗を確認**

Run:
```bash
cd /Users/yui/Documents/workspace/tanaka-yui/learning/06_microservie/bff
go mod tidy
go test ./internal/handler/...
```

Expected: コンパイルエラー（`handler.NewProducts` 未定義）

- [ ] **Step 4: ハンドラ実装**

Create `06_microservie/bff/internal/handler/products.go`:

```go
package handler

import (
	"context"
	"encoding/json"
	"net/http"

	catalogv1 "microservie/proto/gen/go/catalog/v1"
)

type CatalogClient interface {
	ListProducts(ctx context.Context) ([]*catalogv1.Product, error)
}

type Products struct {
	c CatalogClient
}

func NewProducts(c CatalogClient) *Products {
	return &Products{c: c}
}

type productDTO struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	PriceCents  int32  `json:"price_cents"`
}

type listResponse struct {
	Products []productDTO `json:"products"`
}

func (p *Products) List(w http.ResponseWriter, r *http.Request) {
	ps, err := p.c.ListProducts(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	out := listResponse{Products: make([]productDTO, 0, len(ps))}
	for _, x := range ps {
		out.Products = append(out.Products, productDTO{
			ID: x.Id, Name: x.Name, Description: x.Description, PriceCents: x.PriceCents,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}
```

- [ ] **Step 5: テストがパスすることを確認**

Run:
```bash
cd /Users/yui/Documents/workspace/tanaka-yui/learning/06_microservie/bff
go mod tidy
go test ./internal/handler/...
```

Expected: `ok  microservie/bff/internal/handler`

- [ ] **Step 6: コミット**

```bash
cd /Users/yui/Documents/workspace/tanaka-yui/learning
git add 06_microservie/bff/
git commit -m "microservices(bff): add products handler with unit tests"
```

---

### Task 9: BFF gRPC クライアントと main 接続

**Files:**
- Create: `06_microservie/bff/internal/client/catalog.go`
- Create: `06_microservie/bff/internal/obs/otel.go`
- Create: `06_microservie/bff/main.go`

- [ ] **Step 1: catalog gRPC クライアントを作成**

Create `06_microservie/bff/internal/client/catalog.go`:

```go
package client

import (
	"context"

	catalogv1 "microservie/proto/gen/go/catalog/v1"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Catalog struct {
	c catalogv1.CatalogServiceClient
}

func DialCatalog(addr string) (*Catalog, error) {
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
	if err != nil {
		return nil, err
	}
	return &Catalog{c: catalogv1.NewCatalogServiceClient(conn)}, nil
}

func (c *Catalog) ListProducts(ctx context.Context) ([]*catalogv1.Product, error) {
	res, err := c.c.ListProducts(ctx, &catalogv1.ListProductsRequest{Limit: 50})
	if err != nil {
		return nil, err
	}
	return res.Products, nil
}
```

- [ ] **Step 2: BFF 用 OTel 初期化を作成**

Create `06_microservie/bff/internal/obs/otel.go`:

```go
package obs

import (
	"context"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

func InitTracing(ctx context.Context, serviceName string) (func(context.Context) error, error) {
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		endpoint = "otel-collector:4317"
	}
	exp, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}
	res, err := resource.New(ctx, resource.WithAttributes(semconv.ServiceName(serviceName)))
	if err != nil {
		return nil, err
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	return tp.Shutdown, nil
}
```

> 設計書 4.7 「意図的な重複」に基づき、catalog の `obs/otel.go` とほぼ同じ内容をコピーする。

- [ ] **Step 3: `main.go` を作成**

Create `06_microservie/bff/main.go`:

```go
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"microservie/bff/internal/client"
	"microservie/bff/internal/handler"
	"microservie/bff/internal/obs"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger.With("service", "bff"))

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	shutdownTracer, err := obs.InitTracing(ctx, "bff")
	if err != nil {
		slog.Error("init tracing", "err", err)
		os.Exit(1)
	}
	defer func() { _ = shutdownTracer(context.Background()) }()

	catalogAddr := os.Getenv("CATALOG_ADDR")
	if catalogAddr == "" {
		catalogAddr = "catalog:50051"
	}
	cat, err := client.DialCatalog(catalogAddr)
	if err != nil {
		slog.Error("dial catalog", "err", err)
		os.Exit(1)
	}

	products := handler.NewProducts(cat)

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(corsMiddleware)
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	r.Get("/api/products", products.List)

	port := os.Getenv("HTTP_PORT")
	if port == "" {
		port = "8080"
	}
	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           otelhttp.NewHandler(r, "bff"),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, c := context.WithTimeout(context.Background(), 5*time.Second)
		defer c()
		_ = srv.Shutdown(shutdownCtx)
	}()

	slog.Info("bff HTTP server starting", "port", port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("listen", "err", err)
		os.Exit(1)
	}
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:5173")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
```

- [ ] **Step 4: 依存解決とビルド**

Run:
```bash
cd /Users/yui/Documents/workspace/tanaka-yui/learning/06_microservie/bff
go mod tidy
go build -o /tmp/bff-bin .
```

Expected: エラーなくビルド完了

- [ ] **Step 5: コミット**

```bash
cd /Users/yui/Documents/workspace/tanaka-yui/learning
git add 06_microservie/bff/
git commit -m "microservices(bff): wire HTTP server to catalog gRPC + OTel"
```

---

### Task 10: BFF Dockerfile

**Files:**
- Create: `06_microservie/bff/Dockerfile`

- [ ] **Step 1: Dockerfile を作成**

Create `06_microservie/bff/Dockerfile`:

```dockerfile
FROM golang:1.26-bookworm AS builder

WORKDIR /work

COPY proto/go.mod proto/go.sum proto/
COPY proto/gen ./proto/gen

WORKDIR /work/bff
COPY bff/go.mod bff/go.sum ./
RUN go mod download

COPY bff/ ./

RUN CGO_ENABLED=0 go build -o /out/bff .

FROM gcr.io/distroless/static:nonroot
WORKDIR /app
COPY --from=builder /out/bff /app/bff

ENV HTTP_PORT=8080
EXPOSE 8080

ENTRYPOINT ["/app/bff"]
```

- [ ] **Step 2: 単体ビルドを通す**

Run:
```bash
cd /Users/yui/Documents/workspace/tanaka-yui/learning/06_microservie
docker build -f bff/Dockerfile -t bff:dev .
```

Expected: イメージビルドが成功（`Successfully tagged bff:dev`）

- [ ] **Step 3: コミット**

```bash
cd /Users/yui/Documents/workspace/tanaka-yui/learning
git add 06_microservie/bff/Dockerfile
git commit -m "microservices(bff): add multi-stage distroless Dockerfile"
```

---

### Task 11: OTel Collector 設定

**Files:**
- Create: `06_microservie/infra/otel-collector/config.yaml`

- [ ] **Step 1: Collector 設定を作成**

Create `06_microservie/infra/otel-collector/config.yaml`:

```yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
      http:
        endpoint: 0.0.0.0:4318

processors:
  batch:
    timeout: 1s
    send_batch_size: 512

exporters:
  otlp/jaeger:
    endpoint: jaeger:4317
    tls:
      insecure: true
  debug:
    verbosity: basic

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [batch]
      exporters: [otlp/jaeger, debug]
```

- [ ] **Step 2: コミット**

```bash
cd /Users/yui/Documents/workspace/tanaka-yui/learning
git add 06_microservie/infra/
git commit -m "microservices: add OTel Collector config (OTLP -> Jaeger)"
```

---

### Task 12: docker-compose.yml で全コンテナを起動

**Files:**
- Create: `06_microservie/docker-compose.yml`

- [ ] **Step 1: `docker-compose.yml` を作成**

Create `06_microservie/docker-compose.yml`:

```yaml
services:
  postgres-catalog:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: catalog
      POSTGRES_PASSWORD: catalog
      POSTGRES_DB: catalog
    ports:
      - "55432:5432"
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U catalog -d catalog"]
      interval: 2s
      timeout: 2s
      retries: 20

  catalog:
    build:
      context: .
      dockerfile: services/catalog/Dockerfile
    environment:
      DATABASE_URL: postgres://catalog:catalog@postgres-catalog:5432/catalog?sslmode=disable
      OTEL_EXPORTER_OTLP_ENDPOINT: otel-collector:4317
      GRPC_PORT: "50051"
    depends_on:
      postgres-catalog:
        condition: service_healthy
      otel-collector:
        condition: service_started
    ports:
      - "50051:50051"

  bff:
    build:
      context: .
      dockerfile: bff/Dockerfile
    environment:
      CATALOG_ADDR: catalog:50051
      OTEL_EXPORTER_OTLP_ENDPOINT: otel-collector:4317
      HTTP_PORT: "8080"
    depends_on:
      - catalog
    ports:
      - "8080:8080"

  otel-collector:
    image: otel/opentelemetry-collector-contrib:0.110.0
    command: ["--config=/etc/otel/config.yaml"]
    volumes:
      - ./infra/otel-collector/config.yaml:/etc/otel/config.yaml:ro
    depends_on:
      - jaeger
    ports:
      - "4317:4317"
      - "4318:4318"

  jaeger:
    image: jaegertracing/all-in-one:1.62
    environment:
      COLLECTOR_OTLP_ENABLED: "true"
    ports:
      - "16686:16686"
      - "4327:4317"
```

- [ ] **Step 2: 全コンテナ起動**

Run:
```bash
cd /Users/yui/Documents/workspace/tanaka-yui/learning/06_microservie
docker compose up -d --build
```

Expected: すべてのサービスが起動。`docker compose ps` で全 status が `running` / `healthy`

- [ ] **Step 3: 起動状態の確認**

Run:
```bash
cd /Users/yui/Documents/workspace/tanaka-yui/learning/06_microservie
docker compose ps
```

Expected: 5サービス（postgres-catalog / catalog / bff / otel-collector / jaeger）が稼働

- [ ] **Step 4: BFF heath check**

Run:
```bash
curl -s -o /dev/null -w "%{http_code}\n" http://localhost:8080/healthz
```

Expected: `200`

- [ ] **Step 5: 一旦停止**

Run:
```bash
cd /Users/yui/Documents/workspace/tanaka-yui/learning/06_microservie
docker compose down
```

- [ ] **Step 6: コミット**

```bash
cd /Users/yui/Documents/workspace/tanaka-yui/learning
git add 06_microservie/docker-compose.yml
git commit -m "microservices: add docker-compose with catalog/bff/postgres/otel/jaeger"
```

---

### Task 13: Makefile

**Files:**
- Create: `06_microservie/Makefile`

- [ ] **Step 1: Makefile を作成**

Create `06_microservie/Makefile`:

```makefile
.DEFAULT_GOAL := help

.PHONY: help up down logs proto seed test clean

help: ## ヘルプを表示
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-12s\033[0m %s\n", $$1, $$2}'

up: ## 全サービス起動
	docker compose up -d --build

down: ## 停止
	docker compose down

logs: ## 全サービスのログ追従
	docker compose logs -f

proto: ## proto から Go コード再生成
	buf generate

seed: ## 初期データ投入（商品10件）
	docker compose exec -T postgres-catalog psql -U catalog -d catalog -f - < services/catalog/seed/seed.sql

test: ## 各 Go モジュールのユニット/インテグレーションテスト
	cd services/catalog && go test ./...
	cd bff && go test ./...

clean: ## ボリュームも含めて削除
	docker compose down -v
```

- [ ] **Step 2: `make help` で動作確認**

Run:
```bash
cd /Users/yui/Documents/workspace/tanaka-yui/learning/06_microservie
make help
```

Expected: ターゲット一覧が表示される

- [ ] **Step 3: コミット**

```bash
cd /Users/yui/Documents/workspace/tanaka-yui/learning
git add 06_microservie/Makefile
git commit -m "microservices: add Makefile with up/down/logs/proto/seed/test/clean"
```

---

### Task 14: Seed データ投入

**Files:**
- Create: `06_microservie/services/catalog/seed/seed.sql`

- [ ] **Step 1: シード SQL を作成**

Create `06_microservie/services/catalog/seed/seed.sql`:

```sql
TRUNCATE TABLE products;

INSERT INTO products (id, name, description, price_cents) VALUES
  ('p-001', 'Notebook A5',     'A5サイズの方眼ノート',           480),
  ('p-002', 'Ballpoint Pen',   '0.5mm 油性ボールペン 黒',         180),
  ('p-003', 'Mechanical KB',   'メカニカルキーボード 茶軸 65%', 12800),
  ('p-004', 'USB-C Cable',     'USB-C 1m 100W PD対応',           1200),
  ('p-005', 'Coffee Mug',      '陶器マグカップ 350ml',           2500),
  ('p-006', 'Desk Lamp',       'LED デスクライト 調光対応',      5800),
  ('p-007', 'Sticky Notes',    '正方形 75mm 5色アソート',         380),
  ('p-008', 'Tote Bag',        'A4対応 帆布トートバッグ',        3200),
  ('p-009', 'Water Bottle',    'ステンレス 500ml',               2400),
  ('p-010', 'Highlighter Set', '蛍光ペン 6色セット',              420);
```

> 単価は整数の最小通貨単位で持つ（日本円のため `1 = 1円` として扱う）。列名は国際的な慣習に従い `price_cents` のままだが、教材で扱う通貨は円のみ。

- [ ] **Step 2: 起動して seed を実行**

Run:
```bash
cd /Users/yui/Documents/workspace/tanaka-yui/learning/06_microservie
make up
sleep 8
make seed
```

Expected: `INSERT 0 10` の出力

- [ ] **Step 3: products テーブルを直接確認**

Run:
```bash
docker compose -f /Users/yui/Documents/workspace/tanaka-yui/learning/06_microservie/docker-compose.yml \
  exec -T postgres-catalog psql -U catalog -d catalog -c "SELECT count(*) FROM products;"
```

Expected: `count` 列が `10`

- [ ] **Step 4: コミット**

```bash
cd /Users/yui/Documents/workspace/tanaka-yui/learning
git add 06_microservie/services/catalog/seed/
git commit -m "microservices(catalog): add seed SQL with 10 sample products"
```

---

### Task 15: End-to-end 検証と verification log

**Files:**
- Create: `06_microservie/VERIFICATION.md`

- [ ] **Step 1: 起動済みでなければ起動**

Run:
```bash
cd /Users/yui/Documents/workspace/tanaka-yui/learning/06_microservie
docker compose ps | grep -q running || (make up && sleep 10 && make seed)
```

- [ ] **Step 2: BFF 経由で商品一覧を取得**

Run:
```bash
curl -s http://localhost:8080/api/products | head -c 500
```

Expected: `{"products":[{"id":"p-001","name":"Notebook A5",...}` 形式の JSON、10件分

- [ ] **Step 3: ログから trace_id を抽出して相関を確認**

Run:
```bash
docker compose -f /Users/yui/Documents/workspace/tanaka-yui/learning/06_microservie/docker-compose.yml logs bff --tail 5 | grep -oE '"trace_id":"[a-f0-9]+"' | head -1
```

> このコマンドは現状の slog 設定では trace_id を未注入。Task 16 以降の改善対象として VERIFICATION.md にメモ。

- [ ] **Step 4: Jaeger UI を開いて trace を目視確認**

Run:
```bash
open http://localhost:16686
```

ブラウザで以下を実行：
1. Service ドロップダウンで `bff` を選択
2. Find Traces をクリック
3. 直近の `/api/products` リクエストの span を開く
4. `bff → catalog → catalog.ListProducts` の階層が見えることを確認

- [ ] **Step 5: VERIFICATION.md に結果を記録**

Create `06_microservie/VERIFICATION.md`:

```markdown
# Plan 1 (Foundation) Verification Log

## 合格項目

- [x] `make up` で全コンテナが起動・healthy
- [x] `curl http://localhost:8080/api/products` が10件のJSONを返す
- [x] Jaeger UI（http://localhost:16686）で `bff → catalog` のtraceが見える
- [x] `make test` がパス

## Plan 2 で対応する未完事項

- slog ログへの trace_id 注入が未実装（`obs.LogAttrsFromCtx` ヘルパは作ったが、handler / server から呼び出していない）
- Postgres スパンが trace に含まれていない（pgx に OTel 計装を入れる必要あり）
- BFF の `corsMiddleware` は `OTEL` 計装の前に CORS を挟む順序の妥当性要確認

## アクセス先一覧

| URL | 用途 |
|---|---|
| http://localhost:8080/api/products | BFF REST |
| http://localhost:16686 | Jaeger UI |
| postgres://catalog:catalog@localhost:55432/catalog | catalog DB（ローカルクライアントから接続用） |
```

- [ ] **Step 6: `make test` がパスすることを最終確認**

Run:
```bash
cd /Users/yui/Documents/workspace/tanaka-yui/learning/06_microservie
make test
```

Expected: catalog / bff 両モジュールのテストがパス（exit code 0）

- [ ] **Step 7: コミット**

```bash
cd /Users/yui/Documents/workspace/tanaka-yui/learning
git add 06_microservie/VERIFICATION.md
git commit -m "microservices: add Plan 1 verification log and pending items for Plan 2"
```

---

## Plan 1 完了条件チェックリスト

最終的に以下がすべて満たされていることを確認する：

- [ ] `06_microservie/` ディレクトリツリーが揃っている
- [ ] `buf generate` で proto から Go コードが生成できる
- [ ] `make up` で5コンテナ（postgres-catalog / catalog / bff / otel-collector / jaeger）が起動する
- [ ] `make seed` で products テーブルに10件投入される
- [ ] `curl http://localhost:8080/api/products` が JSON で10商品を返す
- [ ] Jaeger UI で `bff → catalog` のサービス間 trace が一本見える
- [ ] `make test` で全テストがパス
- [ ] `make down` で全コンテナが停止する
- [ ] `make clean` で volume を含めて初期化できる

これらが揃った時点で **Plan 2（残りバックエンド + Saga + Resilience）** に進む準備が整う。
