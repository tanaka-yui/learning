# Plan 1 (Foundation) Verification Log

実施日: 2026-05-12
ブランチ: feat/microservices

## 合格項目

- [x] `make up` で全コンテナが起動・healthy
- [x] `make seed` で products 10件投入
- [x] `curl http://localhost:8080/api/products` が 10 件の JSON を返す
- [x] Jaeger に `bff` および `catalog` サービスが現れる
- [x] checkout（ListProducts呼び出し）の trace が `bff → catalog` の階層で見える
- [x] `make test` がパス

## Plan 2 で対応する未完事項

- BFF/catalog の slog ログに trace_id が自動注入されていない
  - `obs.LogAttrsFromCtx` ヘルパは catalog に作ったが、handler/server から呼び出していない
  - Plan 2 で `slog.Default` を Context-aware ハンドラに差し替えて自動注入する
- Postgres スパンが trace に含まれていない（pgx に OTel 計装を追加する必要あり）
- BFF の `corsMiddleware` を OTel 計装の後段に置く順序の妥当性は要確認
- testcontainers を Rancher Desktop で動かすための env var が必要（`DOCKER_HOST` / `TESTCONTAINERS_RYUK_DISABLED`）。現在は Makefile の test ターゲットでのみ自動設定

## アクセス先一覧

| URL | 用途 |
|---|---|
| http://localhost:8080/api/products | BFF REST |
| http://localhost:8080/healthz | BFF ヘルスチェック |
| http://localhost:16686 | Jaeger UI |
| postgres://catalog:catalog@localhost:55432/catalog | catalog DB（ホスト側クライアントから接続） |

## 実行ログ抜粋

### Step 2: `curl http://localhost:8080/api/products`

```json
{"products":[{"id":"p-001","name":"Notebook A5","description":"A5サイズの方眼ノート","price_cents":480},{"id":"p-002","name":"Ballpoint Pen","description":"0.5mm 油性ボールペン 黒","price_cents":180},{"id":"p-003","name":"Mechanical KB","description":"メカニカルキーボード 茶軸 65%","price_cents":12800},{"id":"p-004","name":"USB-C Cable","description":"USB-C 1m 100W PD対応","price_cents":1200},{"id":"p-005","name":"Coffee Mug","description":"陶器マグカップ 350ml","price_cents":2500},{"id":"p-006","name":"Desk Lamp","description":"LED デスクライト 調光対応","price_cents":5800},{"id":"p-007","name":"Sticky Notes","description":"正方形 75mm 5色アソート","price_cents":380},{"id":"p-008","name":"Tote Bag","description":"A4対応 帆布トートバッグ","price_cents":3200},{"id":"p-009","name":"Water Bottle","description":"ステンレス 500ml","price_cents":2400},{"id":"p-010","name":"Highlighter Set","description":"蛍光ペン 6色セット","price_cents":420}]}
```

10件確認。

### Step 3: BFF ログ（trace_id 確認）

```
bff-1  | {"time":"2026-05-12T11:07:27.084790143Z","level":"INFO","msg":"bff HTTP server starting","service":"bff","port":"8080"}
```

BFF の slog ログにリクエスト毎の trace_id は出力されていない。これは Plan 2 で対応する未完事項。

### Step 4: Jaeger API 確認

#### サービス一覧

```
GET http://localhost:16686/api/services
→ {"data":["catalog","bff"],"total":2,"limit":0,"offset":0,"errors":null}
```

`bff` と `catalog` の両サービスが Jaeger に登録されていることを確認。

#### トレース詳細（bff サービス、最新1件）

```
traceID: d76e383b8196913687791fd0473d63ae

processes:
  p1: catalog
  p2: bff

spans:
  spanID=b71cee47b4f9b82f [bff]     op=bff                                   kind=server  parent=[]
  spanID=f4c7204b3b07db67 [bff]     op=catalog.v1.CatalogService/ListProducts kind=client  parent=[b71cee47b4f9b82f]
  spanID=f50d806a2ffe19b3 [catalog] op=catalog.v1.CatalogService/ListProducts kind=server  parent=[f4c7204b3b07db67]
```

`bff (HTTP server span) → bff (gRPC client span) → catalog (gRPC server span)` の
3 スパン階層が 1 つの traceID で伝播していることを確認。

### Step 5: `make test`

```
DOCKER_HOST=unix:///Users/yui/.rd/docker.sock TESTCONTAINERS_RYUK_DISABLED=true sh -c 'cd services/catalog && go test ./...'
?       microservie/catalog                      [no test files]
?       microservie/catalog/internal/obs         [no test files]
ok      microservie/catalog/internal/repo        (cached)
ok      microservie/catalog/internal/server      (cached)
cd bff && go test ./...
?       microservie/bff                          [no test files]
?       microservie/bff/internal/client          [no test files]
ok      microservie/bff/internal/handler         (cached)
?       microservie/bff/internal/obs             [no test files]
```

catalog / bff 両モジュールのテストが全てパス（exit code 0）。
