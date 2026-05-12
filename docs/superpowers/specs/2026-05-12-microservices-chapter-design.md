# 06_microservie 章 設計書

作成日: 2026-05-12
対象ディレクトリ: `/Users/yui/Documents/workspace/tanaka-yui/learning/06_microservie`

---

## 1. 章の全体像と学習目的

### 1.1 この章で学べること

1. マイクロサービスの「考え方」と、選ぶべきとき/避けるべきとき
2. メリット（独立デプロイ・スケール・障害分離）とデメリット（運用複雑性・分散トランザクション・観測性の困難）
3. Conway's law と Team Topologies の対応関係 — なぜ「組織が先、アーキテクチャが後」になりがちか
4. 小規模マイクロサービスの実装パターン（サービス分割・通信プロトコル・データ所有・レジリエンス・観測性）
5. 大規模になったときの課題と、サービスメッシュ（Istio）の位置づけ — コードは書かず概念で理解

### 1.2 読者像

- 02_cache / 05_database を終えた読者を想定
- HTTP/REST と DB の基本は既知
- gRPC・OpenTelemetry は本章で導入する前提

### 1.3 学習動線

1. docs を順に読む（概念 → pros/cons → Conway → 分割 → 通信 → データ所有 → レジリエンス → 観測性 → 大規模/Istio）
2. サンプルを `make up` で起動し、React UI から注文してみる
3. Jaeger UI で trace を見る、`docker compose logs` で trace_id を追う
4. `docs/10_patterns_in_code.md` が「このdocはこのコード箇所」と紐付けて読み直しを支援する

### 1.4 学習スタイル

「動かす」より「読んで・実行して・読み直す」の往復が主。モノリスは実装せず、概念の対比対象としてのみ扱う。

---

## 2. ディレクトリ構成とドキュメント計画

### 2.1 ディレクトリレイアウト

```
06_microservie/
├── README.md                       # 章全体の入口・読む順番・前提知識
├── docs/
│   ├── 01_concepts.md              # マイクロサービスとは/何でないか、モノリス・モジュラーモノリスとの違い
│   ├── 02_pros_cons.md             # メリット・デメリット、選定基準（チェックリスト形式）
│   ├── 03_conway.md                # Conway's law、Inverse Conway Maneuver、Team Topologies（4種のチーム/3種のインタラクション）
│   ├── 04_decomposition.md         # サービス分割の考え方（DDD境界づけられたコンテキスト/ビジネス能力）、ECドメインへの適用例
│   ├── 05_communication.md         # 同期 vs 非同期、REST/gRPC の選択基準、本サンプルでのプロトコル方針
│   ├── 06_data_ownership.md        # DB-per-service、参照整合性が効かない世界、結果整合性
│   ├── 07_resilience.md            # timeout/retry/circuit breaker/Saga(補償)、idempotency。bulkhead 等は概念のみ
│   ├── 08_observability.md         # 構造ログ・trace_id 伝搬・分散トレース、OpenTelemetry の基本
│   ├── 09_scaling_istio.md         # 大規模化の課題、サービスメッシュが解くもの（mTLS / L7ルーティング / 可観測性）、Istio 構成要素
│   └── 10_patterns_in_code.md      # 各パターンとサンプルコード箇所のマッピング（学習動線の架け橋）
├── proto/                          # gRPC 契約（.proto）— buf で管理
│   ├── catalog/v1/catalog.proto
│   ├── inventory/v1/inventory.proto
│   ├── order/v1/order.proto
│   ├── payment/v1/payment.proto
│   └── user/v1/user.proto
├── services/
│   ├── catalog/                    # Go: 商品カタログ
│   ├── inventory/                  # Go: 在庫
│   ├── order/                      # Go: 注文（Saga + resilience の主舞台）
│   ├── payment/                    # Go: 決済（模擬。意図的に時々失敗）
│   └── user-auth/                  # Go: ユーザ/認証（JWT 発行）
├── bff/                            # Go: BFF（REST→gRPC 変換、UI 向けの集約）
├── frontend/                       # React + Vite: 最小UI（商品一覧/カート/注文/履歴）
├── infra/
│   ├── otel-collector/             # OTel Collector 設定
│   └── jaeger/                     # Jaeger 設定
├── docker-compose.yml              # 全コンテナ起動
└── Makefile                        # make up/down/test/trace/proto/clean/demo:*
```

### 2.2 ドキュメント方針

- 既存章（02_cache, 05_database）の文体・構成に揃える（日本語・ASCII図/Mermaid・コードリンク）
- 各 doc は 2,000〜4,000 字目安。長くなりすぎたら分割
- `10_patterns_in_code.md` は「Saga は `services/order/internal/saga/checkout.go:42` を見る」のように行番号レベルで案内する

### 2.3 proto 管理

- `buf` で一元管理。`buf.gen.yaml` から Go コードを生成
- バージョンは `v1` のみ
- React/BFF 間のスキーマは OpenAPI 自動生成は行わず、TypeScript 型を手書きで揃える（教材の見通しを優先）

---

## 3. サンプルアプリのアーキテクチャ

### 3.1 全体図

```
                     ┌──────────────────┐
                     │  React Frontend  │  (Vite + React 18)
                     │  :5173           │
                     └────────┬─────────┘
                              │ REST/JSON（HttpOnly Cookie）
                              ▼
                     ┌──────────────────┐
                     │   BFF (Go)       │  :8080
                     │  - 集約          │
                     │  - 認証検証      │
                     │  - traceparent 開始
                     └────┬──┬──┬──┬────┘
            gRPC          │  │  │  │           gRPC
        ┌────────────────┘  │  │  └────────────────────┐
        ▼                   ▼  ▼                       ▼
 ┌────────────┐     ┌────────────┐     ┌────────────┐     ┌────────────┐
 │  Catalog   │     │  User-Auth │     │   Order    │     │  Inventory │
 │  :50051    │     │   :50052   │     │   :50053   │────▶│  :50054    │
 │  Postgres  │     │  Postgres  │     │  Postgres  │ gRPC└────────────┘
 └────────────┘     └────────────┘     │            │     ┌────────────┐
                                       │            │────▶│  Payment   │
                                       │            │ gRPC│  :50055    │
                                       └────────────┘     │  Postgres  │
                                                          └────────────┘

   全プロセス  ──── OTLP/gRPC ───▶  OTel Collector  ──▶  Jaeger UI (:16686)
                                              └─▶ stdout（構造ログと trace_id 相関）
```

### 3.2 サービス責務と所有データ

| サービス | 責務 | 所有データ（自分の DB スキーマ） | 主な公開 API（gRPC） |
|---|---|---|---|
| **catalog** | 商品マスタ、表示用情報 | `products(id, name, price, ...)` | `ListProducts`, `GetProduct` |
| **inventory** | 在庫数の真の所在、予約・確定・解放 | `stocks(product_id, available, reserved)`、`reservations(id, order_id, ...)` | `Reserve`, `Commit`, `Release` |
| **order** | 注文ライフサイクル、Saga 実行 | `orders(id, user_id, status, ...)`、`order_items(order_id, ...)`、`saga_log(...)` | `PlaceOrder`, `GetOrder`, `ListOrders` |
| **payment** | 決済の試行・結果 | `payments(id, order_id, status, amount, ...)` | `Charge`, `Refund` |
| **user-auth** | ユーザ管理、JWT 発行・検証 | `users(id, email, password_hash, ...)` | `SignUp`, `SignIn`, `ValidateToken` |
| **bff** | UI 向け集約・REST 化 | （永続化なし） | REST: `/api/*` |

### 3.3 重要原則

- 各サービスは **他サービスの DB を直接見ない**。必ず gRPC 越し
- `inventory` と `catalog` が同じ `product_id` を扱うが、片方は「在庫の真実」、もう片方は「カタログの真実」。重複は意図的（マイクロサービスでは正常）
- Order は他サービスのデータをコピーしない（注文行に price snapshot だけ保持）

### 3.4 認証の伝搬方針

- React は HttpOnly Cookie に JWT を保持
- BFF が REST 入口で `user-auth.ValidateToken` を呼び、ユーザ ID を取り出す
- BFF から各バックエンドサービスへの gRPC 呼び出しでは、検証済みの `user_id` を gRPC metadata（`x-user-id`）で伝搬する
- バックエンドサービスは BFF を信頼し、JWT 自体を再検証しない（ゼロトラスト・サービス間 mTLS は `docs/09_scaling_istio.md` の議論対象）
- この単純化は学習用の意図的な選択であり、`docs/08_observability.md` または `docs/09_scaling_istio.md` で明示する

### 3.4 主要フロー: 注文確定（Saga）

```
React → BFF /api/checkout
         BFF → order.PlaceOrder
                  ├─ saga step1: inventory.Reserve(items)     ─┐
                  │     失敗時: order.status = FAILED           │
                  ├─ saga step2: payment.Charge(amount)         │ どこかで失敗したら
                  │     失敗時: inventory.Release ← 補償        │ 補償ステップを逆順に実行
                  ├─ saga step3: inventory.Commit(reservation)  │
                  │     失敗時: payment.Refund ← 補償           │
                  └─ order.status = CONFIRMED                  ─┘
```

- 各 step の呼び出しに timeout + retry
- payment.Charge には circuit breaker（一定失敗率で短時間遮断）
- すべての step は冪等性キー（`reservation_id`, `payment_idempotency_key`）を渡す
- `saga_log` テーブルに各 step の状態を記録（失敗時のリプレイ・診断材料）

---

## 4. レジリエンスと観測性の実装計画

### 4.1 レジリエンスパターン

| パターン | 実装箇所 | 方針 |
|---|---|---|
| **Timeout** | `bff → 各サービス`、`order → inventory/payment` | gRPC Client の Context deadline。サービス側でも `ctx.Err()` チェック。デフォルト 2s |
| **Retry** | `order → inventory.Reserve`、`order → payment.Charge` | 指数バックオフ（`github.com/cenkalti/backoff/v4`）。冪等な呼び出しのみリトライ。最大3回 |
| **Circuit Breaker** | `order → payment.Charge` | `github.com/sony/gobreaker`。直近10秒の失敗率50%超で30秒 Open、半開で1リクエストずつ |
| **Saga（補償）** | `order` サービス内（オーケストレータ方式） | `services/order/internal/saga/checkout.go` に集約。各 step と補償 step を構造体で定義 |

### 4.2 意図的な不安定さ

- `payment` サービスは環境変数 `FLAKE_RATE`（0.0〜1.0）で擬似失敗率を設定可能
- `make demo:retry` は `FLAKE_RATE=0.2` で複数回注文 → リトライが発火しつつ最終的には多くが成功することを観察
- `make demo:circuit` は `FLAKE_RATE=0.6` でサーキットブレーカーの open 遷移を観察

### 4.3 docs/07_resilience.md で扱うが実装しないもの

- Bulkhead（リソース隔離）
- Rate limiting
- Backpressure
- Choreography 型 Saga
- Outbox / Inbox パターン
- 結果整合性・イベントソーシング

### 4.4 観測性スタック

```
各 Go プロセス
  ├─ slog で構造ログ（JSON）→ stdout
  │    必須フィールド: time, level, service, trace_id, span_id, msg, attrs...
  ├─ OpenTelemetry SDK
  │    ├─ Tracer Provider → OTLP/gRPC → OTel Collector
  │    └─ Propagator: W3C TraceContext（traceparent ヘッダ）
  └─ gRPC interceptor で server/client 両側に OTel instrumentation

OTel Collector
  ├─ OTLP receiver
  ├─ Batch processor
  └─ Jaeger exporter → Jaeger :16686
```

### 4.5 実装の要点

- BFF は REST 入口で span を開始（リクエストヘッダに `traceparent` があれば継続）
- gRPC interceptor（`go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc`）でサービス境界を自動 instrumentation
- 各サービスは `slog.With("trace_id", trace.SpanContextFromContext(ctx).TraceID().String())` を共通ミドルウェアで注入
- React 側では traceparent を生成しない（BFF に開始を任せる）

### 4.6 学習者が観察できる体験

- Jaeger UI で「checkout 1回」が `bff → order → inventory → payment → inventory.commit` と一本の trace に並ぶ
- 失敗した checkout は赤い span として残り、エラーメッセージが見える
- `docker compose logs order | grep <trace_id>` でログと trace を突き合わせられる

### 4.7 共通ライブラリ方針

- 共通コードを `services/` 間で共有する `pkg/` のような monorepo 風配置にはしない（マイクロサービスの独立性を尊重する教材意図）
- ただし `proto/` だけは共有（gRPC 契約は共有して当然）
- 観測性のセットアップコードは各サービスにコピペ。**意図的な重複** — `docs/08_observability.md` で「現実には共通ライブラリ化するが、教材ではコピーで学習者が読みやすい形にした」と明示する

---

## 5. フロントエンドとローカル開発体験

### 5.1 Frontend (React) スコープ

| 画面 | 内容 | 叩く BFF エンドポイント |
|---|---|---|
| `/` 商品一覧 | カタログから商品リスト、在庫表示 | `GET /api/products`（内部で catalog + inventory を集約） |
| `/products/:id` 詳細 | 単品詳細 | `GET /api/products/:id` |
| `/cart` カート | クライアント側 state（localStorage） | 永続化なし |
| `/checkout` 注文確認 | カート内容 → 確定 | `POST /api/checkout` |
| `/orders` 注文履歴 | 自分の注文一覧と詳細 | `GET /api/orders`, `GET /api/orders/:id` |
| `/signin` ログイン | メール + パスワード | `POST /api/auth/signin` → JWT を HttpOnly Cookie に保存 |
| `/signup` 登録 | 新規登録 | `POST /api/auth/signup` |

### 5.2 技術選定

- Vite + React 18 + TypeScript
- ルーティング: React Router v6
- データフェッチ: 標準 fetch + 軽量カスタム hook（重い状態管理ライブラリは入れない）
- スタイル: 最小限の CSS（Tailwind は入れない、教材の見通しを優先）
- 認証: HttpOnly Cookie に access token、CSRF 対策は SameSite=Lax
- エラー表示: BFF が返す `{ code, message, trace_id }` を UI に表示 → ユーザが trace_id をコピーして Jaeger で確認できる

### 5.3 Makefile

```
make up              # docker compose up -d
make down            # docker compose down
make logs            # 全サービスのログ
make logs/order      # 特定サービスのログ
make proto           # buf generate（proto から Go コード生成）
make seed            # 初期データ投入（商品10件、ユーザ2件、在庫初期化）
make test            # 各サービスの go test
make demo:happy      # 注文を1件叩いて成功 trace を作る
make demo:retry      # FLAKE_RATE=0.2 で複数注文 → リトライ観察
make demo:circuit    # FLAKE_RATE=0.6 で連続注文 → CB open 観察
make trace           # 直近の trace_id 一覧（Jaeger API から）
make clean           # ボリューム含めて全削除
```

### 5.4 docker-compose の構成

```
services:
  postgres-catalog        # 5サービス分の Postgres（DB分離の見える化）
  postgres-inventory
  postgres-order
  postgres-payment
  postgres-user
  catalog                 # Go
  inventory               # Go
  order                   # Go
  payment                 # Go
  user-auth               # Go
  bff                     # Go
  frontend                # vite dev server（ホットリロード）
  otel-collector
  jaeger
```

### 5.5 DB スキーマ初期化

- 各サービスのリポジトリ配下に `migrations/*.sql` を置く（プレーン SQL、シーケンシャル番号付き）
- 各サービスは起動時に自分の DB に対してマイグレーションを冪等に適用する（`golang-migrate/migrate` をライブラリ利用、もしくはアプリ内で順次 `CREATE TABLE IF NOT EXISTS`）
- `make seed` は全マイグレーション後に商品 10 件・ユーザ 2 件・在庫初期値を投入するスクリプトを叩く

### 5.6 リソース見積もり

- Postgres は `postgres:16-alpine` 共通イメージ、5コンテナでもメモリ合計 〜250MB
- Go サービスは distroless ベース（`01_process_thread/go/Dockerfile` を踏襲）、各イメージ約 20MB
- 開発用マシン（16GB RAM 想定）で現実的に動作

---

## 6. テストと検証

### 6.1 テスト方針

ピラミッドの軽量版。教材として全方向に厚いテストは書かないが、各層で「ここを示したい」というサンプルを残す。

| レベル | 場所 | 内容 | 数の目安 |
|---|---|---|---|
| ユニット | 各サービスの `internal/...` 配下 | Saga の状態遷移、Circuit Breaker の open/half-open/close 遷移、JWT 検証ロジック | サービスあたり 3〜5本 |
| インテグレーション | `services/<name>/integration_test.go` | gRPC サーバを起動して実 Postgres に対して叩く（`testcontainers-go`） | サービスあたり 1〜2本 |
| E2E 相当 | `Makefile` の `make demo:*` | docker compose で全サービス起動状態のシナリオ叩き。アサーションは「Jaeger に trace が出る」「BFF が期待ステータスを返す」程度 | 3 シナリオ（happy / retry / circuit） |

### 6.2 テストを書く目的

- 教材として「Saga は状態遷移を持つ」「CB は内部に状態を持つ」と気づかせるための鏡
- カバレッジ目標は設けない

### 6.3 契約テスト

- 専用ツール（Pact 等）は導入しない
- `docs/05_communication.md` で「proto を共有してビルド時に契約違反を検出する」考え方を解説する程度

### 6.4 章完成の検証チェックリスト

| 項目 | 検証方法 |
|---|---|
| `make up` で全コンテナが healthy になる | `docker compose ps` がすべて `healthy` |
| React UI から商品閲覧 → ログイン → 注文ができる | ブラウザで手動 |
| Jaeger UI に checkout の trace が現れる | `:16686` で確認 |
| `make demo:retry` で `FLAKE_RATE=0.2` 時にリトライが発火しつつ多くが成功する | スクリプトの集計出力 |
| `make demo:circuit` で `FLAKE_RATE=0.6` 時に Circuit Breaker が Open に遷移 | order サービスのログに `[CB] state=open` |
| 各サービスの単体テストが `make test` でパスする | exit code 0 |
| 全 doc が揃い、コードへのリンクが死んでいない | `docs/10_patterns_in_code.md` からのリンク手動チェック |

### 6.5 CI・本番想定

- CI は組まない（教材リポジトリのため）
- 本番デプロイ想定も置かない
- ただし `docs/09_scaling_istio.md` で「k8s 上の Istio に載せると何が変わるか」を概念で解説する

---

## 7. スコープ外（明示的に扱わないこと）

実装も詳細な解説もしないが、必要に応じて docs で「触れる程度」とする項目。

- メッセージング基盤（NATS / Kafka / RabbitMQ）と Choreography 型 Saga
- イベントソーシング・CQRS
- Outbox / Inbox パターン
- Consumer Driven Contract（Pact 等）
- API Gateway 製品（Kong, APISIX, Envoy 単体運用）
- Kubernetes マニフェスト・Helm
- Service Mesh の実装（Istio / Linkerd）の実セットアップ — 概念解説のみ
- マルチテナント・課金・在庫補充ロジック
- フロントの UI/UX の作り込み（最小限）

---

## 8. 完了条件

以下がすべて満たされた状態をこの章の完了とする。

1. `06_microservie/README.md` が存在し、章の入口として機能している
2. `docs/01_concepts.md` 〜 `docs/10_patterns_in_code.md` の全 10 ドキュメントが書かれている
3. `proto/` 配下の `.proto` 5 ファイルが揃い、`make proto` で Go コードが生成できる
4. `services/catalog`, `services/inventory`, `services/order`, `services/payment`, `services/user-auth`, `bff` の 6 つの Go バイナリがビルドできる
5. `frontend/` の React アプリがビルドでき、`make up` で `:5173` から開ける
6. `docker-compose.yml` で全サービスが起動し、Jaeger UI が `:16686` で見える
7. 6.4 のチェックリストすべてに合格
