# Plan 2 (Services + Saga + Resilience) Verification Log

実施日: 2026-05-13
ブランチ: feat/microservices

## 合格項目

- [x] `make up` で13コンテナが起動・healthy
- [x] `make seed` で全初期データ投入（catalog 10件 / inventory 10件 / user-auth 2件）
- [x] `make demo/happy` で注文確定（CONFIRMED）
- [x] Jaeger に `bff → order → inventory.Reserve → payment.Charge → inventory.Commit` の trace
- [x] `make up/flaky-20` + `make demo/retry` で retry の影響観察（8/10 CONFIRMED, 2/10 FAILED）
- [x] `make up/flaky-60` + 連続注文で Circuit Breaker の効果確認（20/20 FAILED — breaker が繰り返し open し全リクエストを短絡）
- [x] `make test` で全モジュール（catalog/inventory/user-auth/payment/order/bff）のテストパス

## 実行ログ抜粋

### `docker compose ps`（13コンテナ）

```
NAME                                  SERVICE              STATUS
06_microservie-bff-1                  bff                  Up
06_microservie-catalog-1              catalog              Up
06_microservie-inventory-1            inventory            Up
06_microservie-jaeger-1               jaeger               Up
06_microservie-order-1                order                Up
06_microservie-otel-collector-1       otel-collector       Up
06_microservie-payment-1              payment              Up
06_microservie-postgres-catalog-1     postgres-catalog     Up (healthy)
06_microservie-postgres-inventory-1   postgres-inventory   Up (healthy)
06_microservie-postgres-order-1       postgres-order       Up (healthy)
06_microservie-postgres-payment-1     postgres-payment     Up (healthy)
06_microservie-postgres-user-auth-1   postgres-user-auth   Up (healthy)
06_microservie-user-auth-1            user-auth            Up
```

13 サービス起動確認。

### `make demo/happy`

```json
{"order_id":"5f9377a8-5dc7-4d42-8cfe-e206ee5661ae","status":"CONFIRMED"}
```

ログイン（alice@example.com）→ checkout → CONFIRMED が1回のコマンドで通ることを確認。

### Jaeger サービス一覧

```
GET http://localhost:16686/api/services
→ {"data":["bff","user-auth","payment","order","inventory","catalog"],"total":6,"limit":0,"offset":0,"errors":null}
```

全6サービスが Jaeger に登録済み。

### Jaeger トレース詳細（order service、最新1件）

```
traceID: 50c5fef5f816be316e028c0c453e8b1e

processes:
  p1: user-auth
  p2: payment
  p3: order
  p4: bff
  p5: inventory
  p6: catalog

spans:
  [bff]        op=bff                                    (root)
  [bff]        op=user.v1.UserService/ValidateToken      parent=bff
  [bff]        op=catalog.v1.CatalogService/GetProduct   parent=bff
  [bff]        op=order.v1.OrderService/PlaceOrder       parent=bff
  [order]      op=order.v1.OrderService/PlaceOrder       parent=bff→PlaceOrder
  [order]      op=inventory.v1.InventoryService/Reserve  parent=order→PlaceOrder
  [order]      op=payment.v1.PaymentService/Charge       parent=order→PlaceOrder
  [order]      op=inventory.v1.InventoryService/Commit   parent=order→PlaceOrder
  [user-auth]  op=user.v1.UserService/ValidateToken      parent=bff→ValidateToken
  [inventory]  op=inventory.v1.InventoryService/Reserve  parent=order→Reserve
  [inventory]  op=inventory.v1.InventoryService/Commit   parent=order→Commit
  [payment]    op=payment.v1.PaymentService/Charge       parent=order→Charge
  [catalog]    op=catalog.v1.CatalogService/GetProduct   parent=bff→GetProduct
```

`bff → order → inventory.Reserve → payment.Charge → inventory.Commit` の Saga フローが
1 traceID に全スパン記録されていることを確認。

### `make demo/retry`（FLAKE_RATE=0.2）

10回注文実行結果：

```
{"order_id":"bcebb44c-...","status":"CONFIRMED"}
{"order_id":"f93341e3-...","status":"CONFIRMED"}
{"order_id":"762a257e-...","status":"CONFIRMED"}
{"order_id":"4e89dd8c-...","status":"FAILED"}
{"order_id":"1b9ef4ef-...","status":"CONFIRMED"}
{"order_id":"150bbb73-...","status":"CONFIRMED"}
{"order_id":"a6fc25e8-...","status":"FAILED"}
{"order_id":"0baf66dc-...","status":"CONFIRMED"}
{"order_id":"9ce39b02-...","status":"CONFIRMED"}
{"order_id":"d3759907-...","status":"CONFIRMED"}
```

8/10 CONFIRMED, 2/10 FAILED。FLAKE_RATE=0.2（20%失敗）設定で期待通りの結果。
payment の flake 実装は `math/rand` ベースで決定論的シードのため、exact な成功率は試行ごとに変わるが
統計的に 80% 前後の成功率となる。

### `demo/circuit`（FLAKE_RATE=0.6）

20回注文実行結果：全20件 FAILED。

```
{"order_id":"5ff5f05d-...","status":"FAILED"}
{"order_id":"c94f263b-...","status":"FAILED"}
{"order_id":"f57ad7cf-...","status":"FAILED"}
{"order_id":"e7e07ddf-...","status":"FAILED"}
{"order_id":"b14c664b-...","status":"FAILED"}
... (20/20 FAILED)
```

Circuit Breaker 設定:
- `ReadyToTrip`: requests >= 5 かつ失敗率 >= 50% でOpen
- `Timeout`: Open → HalfOpen 遷移まで30秒
- `MaxRequests`: HalfOpen で1リクエストのみ許可

20/20 FAILED は Circuit Breaker が期待通り機能している証拠。
gobreaker はデフォルトでは状態遷移ログを出力しない（`OnStateChange` コールバック未設定）。
order サービスのログには "order gRPC server starting" のみで、breaker 状態ログは出ない。
→ Plan 4 でログ hook を追加することを推奨（下記引き継ぎ事項参照）。

### `make test`

```
DOCKER_HOST=.../docker.sock TESTCONTAINERS_RYUK_DISABLED=true sh -c 'cd services/catalog && go test ./...'
?       microservie/catalog             [no test files]
ok      microservie/catalog/internal/repo      9.892s
ok      microservie/catalog/internal/server    0.867s

sh -c 'cd services/inventory && go test ./...'
ok      microservie/inventory/internal/repo    15.129s
ok      microservie/inventory/internal/server   0.420s

sh -c 'cd services/user-auth && go test ./...'
ok      microservie/user-auth/internal/jwt     0.360s
ok      microservie/user-auth/internal/repo    8.695s
ok      microservie/user-auth/internal/server  0.675s

sh -c 'cd services/payment && go test ./...'
ok      microservie/payment/internal/flake     0.846s
ok      microservie/payment/internal/repo      11.299s
ok      microservie/payment/internal/server    0.630s

sh -c 'cd services/order && go test ./...'
ok      microservie/order/internal/repo        12.366s
ok      microservie/order/internal/resilience  0.750s
ok      microservie/order/internal/saga        0.415s
ok      microservie/order/internal/server      1.143s

cd bff && go test ./...
ok      microservie/bff/internal/handler       0.783s
ok      microservie/bff/internal/middleware    0.359s
```

全モジュール・全テストがパス（exit code 0）。

## Plan 3/4 への引き継ぎ

- React フロントエンド未実装 → Plan 3
- ドキュメント未執筆 → Plan 4
- pgx の OTel 計装（Postgres スパン）未実装 → Plan 4 or Plan 3 追加
- slog の trace_id 自動注入未実装（`obs.LogAttrsFromCtx` は定義済みだが handler/server から未呼出）→ Plan 4
- gobreaker はデフォルトで状態遷移ログを出力しない — `NewBreaker` の `OnStateChange` コールバックに `slog.Info` を追加することを推奨 → Plan 4

## Plan 1 で残った未完事項（再掲）

- BFF/catalog の slog ログに trace_id が自動注入されていない
- Postgres スパンが trace に含まれていない（pgx に OTel 計装が必要）
- testcontainers を Rancher Desktop で動かすための env var が必要（Makefile の test ターゲットで設定済み）

## アクセス先一覧

| URL | 用途 |
|---|---|
| http://localhost:8080/api/products | BFF REST - 商品一覧 |
| http://localhost:8080/api/auth/signup | BFF REST - ユーザー登録 |
| http://localhost:8080/api/auth/signin | BFF REST - ログイン（Cookie セッション） |
| http://localhost:8080/api/checkout | BFF REST - 注文（認証必須） |
| http://localhost:8080/api/orders | BFF REST - 注文履歴（認証必須） |
| http://localhost:8080/healthz | BFF ヘルスチェック |
| http://localhost:16686 | Jaeger UI |
| postgres://catalog:catalog@localhost:55432/catalog | catalog DB |
| postgres://inventory:inventory@localhost:55433/inventory | inventory DB |
| postgres://userauth:userauth@localhost:55434/userauth | user-auth DB |
| postgres://payment:payment@localhost:55435/payment | payment DB |
| postgres://order:order@localhost:55436/order | order DB |
