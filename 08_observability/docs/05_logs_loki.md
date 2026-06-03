# 05_logs_loki: 構造化ログと Loki

## 構造化ログとは

**構造化ログ**は、ログをキーと値のペアからなる機械可読なデータとして出力する手法だ。テキストログが `"ERROR: request failed"` という文字列を出力するのに対し、構造化ログは `{"level":"ERROR","msg":"request failed","path":"/api/checkout","trace_id":"4bf92f..."}` のように各フィールドが分離された形で記録される。

これにより以下が実現できる。

- ログ集約基盤(Loki 等)での **フィールド単位の検索**
- **trace_id を軸にした他シグナルとの相関**
- 障害調査時の条件絞り込みの高速化

Go 1.21 で標準化された `log/slog` はキー・バリュー形式の構造化ログを標準で提供する。

---

## なぜ trace_id をログに埋めるか

観測性の3本柱(Metrics / Traces / Logs)は、それぞれ単体でも有用だが **接着剤** がなければ別世界のデータになる。trace_id はその接着剤だ。

```
アラート発火 (Metrics)
  ↓ 「どのリクエストでエラーが出たか？」
Traces で該当リクエストの経路を確認
  ↓ 「この span の詳細なコンテキストは？」
Logs で trace_id を使って当該ログ行を抽出
  ↓ パラメータ・例外スタックトレースを読む
```

`trace_id` がログに入っていれば、Grafana の Loki → Tempo ジャンプが1クリックで行える。入っていなければこの経路が断たれる。

---

## slog + otelslog ブリッジの仕組み

```go
// main.go (抜粋)
logger := slog.New(otelslog.NewHandler(serviceName, otelslog.WithLoggerProvider(lp)))
slog.SetDefault(logger)
```

`otelslog.NewHandler` は `log/slog` の `Handler` インタフェースを実装し、ログレコードを OTel の `LogRecord` に変換して `LoggerProvider` に渡す。

重要なのは **context を渡して slog を呼ぶ**ことだ。

```go
// context に span が入っていれば trace_id / span_id が自動付与される
slog.InfoContext(ctx, "checkout requested", "path", r.URL.Path)
```

`slog.Info(...)` のように context なしで呼ぶと trace_id は付与されない。context を使う `slog.InfoContext` / `slog.ErrorContext` を徹底することで、span が存在するすべてのログ行に `trace_id` と `span_id` が入る。

---

## ログの経路: Go → Collector → Loki

```
[Go API]
  slog.InfoContext(ctx, ...) → otelslog → LoggerProvider
       └─ OTLP/gRPC logs → [OTel Collector]
                                └─ otlphttp/loki exporter
                                     └─ http://loki:3100/otlp
                                          └─ [Loki 3.x]
                                               ▲ query
                                          [Grafana :3001]
```

Collector の logs パイプラインは `otlphttp/loki` exporter を使い、Loki の OTLP ネイティブ ingest エンドポイント (`/otlp`) に転送する。Loki 3.x は OTLP を直接受け取れるため、Promtail や Fluentd のようなエージェントは不要だ。

```yaml
# Collector config (logs pipeline, 抜粋)
exporters:
  otlphttp/loki:
    endpoint: http://loki:3100/otlp

service:
  pipelines:
    logs:
      receivers: [otlp]
      processors: [batch]
      exporters: [otlphttp/loki]
```

---

## Loki の構造化 metadata

Loki 3.x は OTLP ingest において `trace_id` / `span_id` / `service_name` / `severity_text` を **structured metadata** として保存する。これにより `allow_structured_metadata: true` の設定のもとで、これらのフィールドがラベルとは別の検索可能なメタデータとして Loki に格納される。

確認済みの structured metadata フィールド:

| フィールド | 内容 |
|---|---|
| `trace_id` | OTel の trace_id (hex 32文字) |
| `span_id` | OTel の span_id (hex 16文字) |
| `service_name` | resource 属性の `service.name` |
| `severity_text` | ログレベル (INFO / ERROR 等) |

---

## Loki のラベル設計とカーディナリティ

Loki はラベルをインデックスキーとして使い、ラベルの組み合わせごとにストリームを管理する。**ラベルのカーディナリティ** (値の種類数) が多すぎるとストリーム数が爆発し、メモリ・クエリ性能が著しく劣化する。

### NG パターン

```
# request_id や trace_id をラベルにしてはいけない
{trace_id="4bf92f3577b34da6a3ce929d0e0e4736"}  ← NG
```

値が無限に増えるフィールドをラベルに使うとカーディナリティが爆発する。

### OK パターン

```
# 低カーディナリティのフィールドだけをラベルにする
{service_name="checkout-api"}              # OK
{service_name="checkout-api"} | json       # メッセージ内フィールドで絞る
```

`trace_id` は structured metadata として保持し、ラベルには含めない。LogQL の `| json` や `| logfmt` パーサーで行内容からフィールドを抽出してフィルタする。

---

## LogQL 例

```logql
# checkout-api のログを全件取得
{service_name="checkout-api"}

# JSON パースして level が ERROR のものを絞る
{service_name="checkout-api"} | json | level = "ERROR"

# trace_id が入っているログ行だけ
{service_name="checkout-api"} | json | trace_id != ""

# path フィールドで絞る
{service_name="checkout-api"} | json | path = "/api/checkout"
```

### curl での確認

```bash
# Loki API でログを検索
curl -s -G 'http://localhost:3100/loki/api/v1/query_range' \
  --data-urlencode 'query={service_name="checkout-api"}'
```

---

## logs ⇄ traces の往復

### Grafana Explore でのワークフロー

1. **Loki** でサービスのログを検索する: `{service_name="checkout-api"} | json`
2. エラーログ行の `trace_id` フィールドをクリックすると **Tempo** にジャンプする
3. Tempo のスパンツリーで遅延・エラーの原因 span を特定する

### 逆方向 (Traces → Logs)

Tempo でトレースを開き「Logs for this span」リンクをクリックすると、同じ `trace_id` を持つ Loki のログ行に飛ぶ。この双方向ジャンプは Grafana のデータソース相関設定 (`tracesToLogsV2`, `derivedFields`) によって実現している (詳細は [06_grafana_correlation.md](./06_grafana_correlation.md))。

---

## まとめ / 関連 doc

- 構造化ログはフィールド単位の検索と trace_id による相関を可能にする。
- `slog.InfoContext(ctx, ...)` のように context を渡すことで trace_id が自動付与される。
- Loki のラベルは低カーディナリティのフィールドに限定し、trace_id は structured metadata として保持する。
- `{service_name="checkout-api"} | json` が基本クエリ。trace_id クリックで Tempo にジャンプできる。

**関連 doc:**
- [01_concepts.md](./01_concepts.md) — 3本柱の概念と相関の考え方
- [02_otel_sdk_go.md](./02_otel_sdk_go.md) — otelslog ブリッジと slog.InfoContext の実装
- [03_traces_e2e.md](./03_traces_e2e.md) — trace_id が span を跨いで伝播する仕組み
- [06_grafana_correlation.md](./06_grafana_correlation.md) — Loki derivedFields と Tempo へのジャンプ設定
- [07_collector.md](./07_collector.md) — Collector の logs パイプライン設定
