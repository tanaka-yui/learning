# 03_otel_sdk_go: OTel Go SDK — 3プロバイダの初期化

## 概要

本章の Go API (`app/internal/obs/otel.go`) は OpenTelemetry Go SDK で **Tracer / Meter / Logger の3プロバイダ** を初期化し、それぞれをグローバルに登録する。06章(マイクロサービス)が traces のみだったのに対し、本章はメトリクスとログも追加している。

---

## 3プロバイダの関係

```
InitTelemetry()
  ├─ TracerProvider  → otel.SetTracerProvider(tp)     // グローバル登録
  ├─ MeterProvider   → otel.SetMeterProvider(mp)      // グローバル登録
  └─ LoggerProvider  → 呼び元に返す(slog ブリッジ用)
```

グローバル登録することで、アプリの任意の場所から `otel.Tracer(...)` や `otel.GetMeterProvider().Meter(...)` でプロバイダを取得できる。LoggerProvider だけはグローバル API が存在しないため、戻り値で渡して main.go の slog ブリッジに渡す設計になっている。

---

## `InitTelemetry` の実装

```go
// app/internal/obs/otel.go (抜粋)

func InitTelemetry(ctx context.Context, serviceName string) (*sdklog.LoggerProvider, func(context.Context) error, error) {
    endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
    if endpoint == "" {
        endpoint = "otel-collector:4317"
    }

    // resource: service.name を全シグナルに付与
    res, err := resource.New(ctx, resource.WithAttributes(semconv.ServiceName(serviceName)))

    // Traces: OTLP gRPC exporter → TracerProvider
    traceExp, _ := otlptrace.New(ctx, otlptrace.WithEndpoint(endpoint), otlptrace.WithInsecure())
    tp := sdktrace.NewTracerProvider(sdktrace.WithBatcher(traceExp), sdktrace.WithResource(res))
    otel.SetTracerProvider(tp)
    otel.SetTextMapPropagator(propagation.TraceContext{})  // W3C traceparent

    // Metrics: OTLP gRPC exporter → MeterProvider (PeriodicReader)
    metricExp, _ := otlpmetric.New(ctx, otlpmetric.WithEndpoint(endpoint), otlpmetric.WithInsecure())
    mp := sdkmetric.NewMeterProvider(
        sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExp)),
        sdkmetric.WithResource(res),
    )
    otel.SetMeterProvider(mp)

    // Logs: OTLP gRPC exporter → LoggerProvider
    logExp, _ := otlplog.New(ctx, otlplog.WithEndpoint(endpoint), otlplog.WithInsecure())
    lp := sdklog.NewLoggerProvider(
        sdklog.WithProcessor(sdklog.NewBatchProcessor(logExp)),
        sdklog.WithResource(res),
    )

    shutdown := func(ctx context.Context) error {
        return errors.Join(tp.Shutdown(ctx), mp.Shutdown(ctx), lp.Shutdown(ctx))
    }
    return lp, shutdown, nil
}
```

---

## resource と semconv.ServiceName

`resource.New` で作成した `res` は3プロバイダすべてに渡す。これにより Tempo・Prometheus・Loki のいずれのデータにも `service.name=checkout-api` が付与され、Grafana で横断検索が可能になる。

semconv のバージョンは `go.opentelemetry.io/otel/semconv/v1.26.0` を使用している。

---

## OTLP gRPC exporter の設定

| オプション | 値 | 意味 |
|---|---|---|
| `WithEndpoint(endpoint)` | `otel-collector:4317` (デフォルト) | Collector の gRPC アドレス |
| `WithInsecure()` | — | TLS なし(ローカル/Docker 内通信) |

エンドポイントは環境変数 `OTEL_EXPORTER_OTLP_ENDPOINT` で上書き可能だ。Docker Compose 外からテストする場合は `localhost:4319` に変更する。

---

## W3C TraceContext propagator

```go
otel.SetTextMapPropagator(propagation.TraceContext{})
```

この1行がフロントエンド→バックエンドの **一貫トレース** の鍵だ。ブラウザが `traceparent` ヘッダを付与したリクエストを Go の `otelhttp` が受け取ると、同じ propagator を使ってヘッダを解析し、受信 span を既存 trace に **継続(continue)** させる。propagator を設定しないと、バックエンド側で新しい trace が独立して生成されてしまう。

---

## メトリクスのエクスポート間隔

`sdkmetric.NewPeriodicReader(metricExp)` のデフォルトエクスポート間隔は 60 秒だ。本章の Docker Compose では環境変数で短縮している。

```yaml
# docker-compose.yml (app サービス)
environment:
  OTEL_METRIC_EXPORT_INTERVAL: "10000"  # 10秒
```

---

## HTTP サーバの自動計装

```go
// main.go
root := red(otelhttp.NewHandler(mux, "http"))
```

`otelhttp.NewHandler` はすべての HTTP リクエストに対して自動的に span を開始・終了する。ハンドラ名(`"http"`)は span の `http.server_name` 属性になる。

### 内部 span の手動追加

```go
func instrumentedCheckout(h http.Handler) http.Handler {
    tracer := otel.Tracer(serviceName)
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        ctx, span := tracer.Start(r.Context(), "checkout.process")
        defer span.End()
        slog.InfoContext(ctx, "checkout requested", "path", r.URL.Path)
        h.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

`tracer.Start(r.Context(), "checkout.process")` は `otelhttp` が作成した親 span の子 span として記録される。Tempo では1つのトレース内に `http <method> <route>` → `checkout.process` の入れ子が見える。

---

## otelslog ブリッジ — ログに trace_id を載せる

```go
// main.go
logger := slog.New(otelslog.NewHandler(serviceName, otelslog.WithLoggerProvider(lp)))
slog.SetDefault(logger)
```

`otelslog.NewHandler` は `log/slog` のバックエンドとして OTel LoggerProvider を使用する。重要なのは `slog.InfoContext(ctx, ...)` のように **context を渡す**ことだ。ctx に span が入っていれば `trace_id` と `span_id` が自動でログフィールドに付与される。Loki でログを見ると各行に `trace_id` フィールドが含まれ、そのまま Tempo へジャンプできる。

---

## RED ミドルウェア (`app/internal/obs/metrics.go`)

```go
meter := otel.GetMeterProvider().Meter(name)
reqs, _ := meter.Int64Counter("http.server.requests", ...)
errs, _ := meter.Int64Counter("http.server.errors", ...)
dur,  _ := meter.Float64Histogram("http.server.duration", metric.WithUnit("ms"), ...)
```

各リクエスト完了後にステータスコードを記録し、`http.route` と `http.status_code` を属性として付与する。Collector が Prometheus 形式に変換した後の実際のメトリクス名は以下の通りだ。

| OTel 名 | Prometheus 名 |
|---|---|
| `http.server.requests` | `http_server_requests_total` |
| `http.server.errors` | `http_server_errors_total` |
| `http.server.duration` | `http_server_duration_milliseconds_{bucket,sum,count}` |

---

## 06章との差分

| 項目 | 06章(traces only) | 本章 |
|---|---|---|
| TracerProvider | あり | あり |
| MeterProvider | なし | あり(PeriodicReader) |
| LoggerProvider | なし | あり(BatchProcessor) |
| slog ブリッジ | なし | あり(trace_id 自動付与) |
| RED ミドルウェア | なし | あり |

---

## まとめ / 関連 doc

- `InitTelemetry` は traces・metrics・logs の3プロバイダを1関数で初期化し、グローバル登録する。
- `propagation.TraceContext{}` の設定が W3C traceparent 伝播の前提条件だ。
- `otelslog.NewHandler` + `slog.InfoContext(ctx, ...)` でログに trace_id が自動付与される。

**関連 doc:**
- [01_concepts.md](./01_concepts.md) — 3本柱の概念とスタック全体像
- [04_traces_e2e.md](./04_traces_e2e.md) — ブラウザ→Go の一貫トレース詳細
- [08_collector.md](./08_collector.md) — Collector 側の受け取り設定
