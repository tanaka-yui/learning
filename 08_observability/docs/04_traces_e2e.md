# 04_traces_e2e: ブラウザ→Go のエンドツーエンドトレース

## span / trace / trace context とは

- **span**: 単一の処理単位。開始時刻・終了時刻・属性・イベントを持つ。
- **trace**: span のツリー。最上位を root span、配下を child span と呼ぶ。同一 trace 内の全 span が同じ `trace_id` を共有する。
- **trace context**: trace_id と span_id をリクエスト境界を越えて伝播させる仕組み。HTTP ではヘッダで受け渡す。

---

## W3C traceparent ヘッダの形式

```
traceparent: 00-<trace_id(32hex)>-<parent_span_id(16hex)>-<flags(2hex)>
```

例:
```
traceparent: 00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01
             └─ version(00)
                └──────────────────────────── trace_id (128bit)
                                             └──────────── parent span_id (64bit)
                                                           └─ flags (01=sampled)
```

Go の `otelhttp` は受信リクエストでこのヘッダを読み取り、同一 trace_id で span を継続させる。これにより Tempo 上でブラウザ発の root span とサーバ側の child span が1本のトレースとして表示される。

---

## ブラウザ→Go の一貫トレースフロー

```
[ブラウザ (React)]
  1. FetchInstrumentation が fetch() をパッチ
  2. "checkout-frontend" の root span を生成
  3. traceparent ヘッダを HTTP リクエストに付与
       ↓ traceparent: 00-<trace_id>-<frontend_span_id>-01
[Go API (otelhttp)]
  4. otelhttp.NewHandler がヘッダを受信
  5. propagation.TraceContext{} で trace_id / parent_span_id を抽出
  6. 同一 trace_id で "http POST /api/checkout" span を開始 (child)
  7. instrumentedCheckout で "checkout.process" span を開始 (child of 6)
  8. slog.InfoContext(ctx, ...) → ログに trace_id が入る
       ↓ OTLP/gRPC
[OTel Collector]
  9. traces パイプライン → Tempo
[Grafana / Tempo]
 10. 1つの trace_id で4 span (browser + server_root + checkout + 内部) が見える
```

---

## フロントエンド側の設定 (`frontend/src/otel.ts`)

```typescript
const provider = new WebTracerProvider({
  resource: resourceFromAttributes({ [ATTR_SERVICE_NAME]: "checkout-frontend" }),
  spanProcessors: [
    new BatchSpanProcessor(new OTLPTraceExporter({ url: `${COLLECTOR}/v1/traces` })),
  ],
});

provider.register({ contextManager: new ZoneContextManager() });

registerInstrumentations({
  instrumentations: [
    new FetchInstrumentation({
      propagateTraceHeaderCorsUrls: [new RegExp(API_BASE)],
    }),
  ],
});
```

デフォルトの propagator は W3C TraceContext であり、`traceparent` ヘッダを自動で付与する。

---

## `propagateTraceHeaderCorsUrls` が必要な理由

ブラウザのクロスオリジン fetch は、デフォルトでは `traceparent` のようなカスタムヘッダを送信しない。`FetchInstrumentation` の `propagateTraceHeaderCorsUrls` に API のオリジンを正規表現で指定することで、そのオリジンへの fetch に限って `traceparent` ヘッダを付与できるようになる。

合わせて Collector 側でも CORS を許可する必要がある。

```yaml
# infra/otel-collector/config.yaml
receivers:
  otlp:
    protocols:
      http:
        endpoint: 0.0.0.0:4318
        cors:
          allowed_origins:
            - "http://localhost:5174"
```

ブラウザは `http://localhost:5174` から Collector の `http://localhost:4320` (→コンテナ 4318) に OTLP/HTTP で span を送る。この CORS 設定がないとブラウザの preflight request が失敗し、フロント側の span が一切 Collector に届かない。

---

## よくある失敗パターン

| 症状 | 原因 | 対処 |
|---|---|---|
| ブラウザ span が Tempo に出ない | Collector の CORS 未設定でブラウザが preflight を弾く | `allowed_origins` に Vite の origin を追加 |
| ブラウザとサーバの span が別 trace になる | `propagation.TraceContext{}` 未設定でサーバが traceparent を無視する | `otel.SetTextMapPropagator(propagation.TraceContext{})` を追加 |
| フロントの span のみ trace が切れる | `propagateTraceHeaderCorsUrls` に API の URL を指定していない | 正規表現で API_BASE を含める |
| Grafana でログから Tempo にジャンプできない | ログに trace_id フィールドがない | `slog.InfoContext(ctx, ...)` で context を渡す |

---

## Tempo でトレースを読む手順

### Grafana Explore から見る

1. Grafana (`http://localhost:3001`) を開く。
2. 左メニュー「Explore」→ データソース「Tempo」を選択。
3. 「Search」タブでサービス名 `checkout-api` または `checkout-frontend` を選択し「Run query」。
4. 一覧からトレースをクリックするとスパンツリーが表示される。

### curl で直接確認する

```bash
# Tempo の HTTP API でトレースを取得
curl http://localhost:3200/api/traces/<traceID>
```

`<traceID>` は Grafana の Explore 画面か、Collector の debug exporter のログから取得できる。レスポンスは Jaeger JSON 形式で返ってくる。

### ログ→トレース連携を確認する

```bash
# Loki でログを検索 (LogQL)
{service_name="checkout-api"} | json | trace_id != ""
```

ログ行の `trace_id` フィールドをクリックすると Tempo の該当トレースにジャンプできる。

---

## まとめ / 関連 doc

- W3C `traceparent` ヘッダが trace_id を HTTP 境界を越えて伝播させる。
- ブラウザ側は `propagateTraceHeaderCorsUrls` の設定、サーバ側は `propagation.TraceContext{}` の設定が両方揃って初めて一貫トレースが成立する。
- Collector の CORS 設定はブラウザ span の送信を可能にするために必須だ。

**関連 doc:**
- [01_concepts.md](./01_concepts.md) — 3本柱の概念とスタック全体像
- [03_otel_sdk_go.md](./03_otel_sdk_go.md) — Go 側の propagator 設定詳細
- [08_collector.md](./08_collector.md) — Collector の CORS 設定と OTLP receiver
