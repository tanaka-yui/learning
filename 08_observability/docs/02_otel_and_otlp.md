# 02_otel_and_otlp: そもそも OpenTelemetry とは — 仕様とプロトコル(OTLP)

[01_concepts.md](./01_concepts.md) では「観測性」という性質と3本柱を扱った。本 doc はその実現手段である **OpenTelemetry(OTel)** そのものを掘り下げる。「何を標準化したプロジェクトなのか」「テレメトリはどんなデータ構造で、どんなプロトコルで運ばれるのか」を押さえると、以降の実装 doc(03〜)が「仕様のどの部分を呼んでいるか」として読めるようになる。

---

## 1. そもそも OpenTelemetry とは

OpenTelemetry は **テレメトリ(traces / metrics / logs)の生成・収集・送信を標準化する、ベンダ中立のオープンソース仕様と実装の集合**だ。CNCF(Cloud Native Computing Foundation)のプロジェクトで、Kubernetes に次いで活発とされる。

### 成り立ち

かつて分散トレースの計装ライブラリは **OpenTracing**(API標準)と **OpenCensus**(Google発のSDK)に二分され、互換性がなかった。2019年に両者が統合して OpenTelemetry が生まれた。「計装の書き方が乱立し、特定ベンダの SDK に縛られる」問題を、業界横断の標準で解消するのが狙いである。

### OTel が標準化するもの

OpenTelemetry は単一のライブラリではなく、**複数の仕様(specification)とその言語別実装**からなる。

| 構成要素 | 役割 |
|---|---|
| **API** | 計装コードが呼ぶインターフェース(`Tracer`, `Meter`, `Logger`)。実装に依存しない。 |
| **SDK** | API の具体実装。サンプリング・バッチ処理・エクスポートを担う。 |
| **データモデル** | テレメトリの共通構造(Resource / Scope / Span / Metric / LogRecord)。 |
| **セマンティック規約** | 属性名の辞書(`service.name`, `http.request.method` など)。 |
| **OTLP** | テレメトリを運ぶワイヤプロトコル(OpenTelemetry Protocol)。 |
| **Collector** | テレメトリを受信・加工・転送する独立プロセス([08_collector.md](./08_collector.md))。 |

ポイントは **「仕様」と「実装」が分離している**ことだ。仕様は言語非依存で定義され、Go・JS・Java など各言語がそれに準拠した SDK を提供する。本章の Go([03_otel_sdk_go.md](./03_otel_sdk_go.md))とブラウザ JS([04_traces_e2e.md](./04_traces_e2e.md))は、同じ仕様の別実装にすぎない。

### API と SDK を分ける理由

ライブラリ作者は **API だけ**に依存して計装を書ける。アプリ側が SDK を入れなければ計装は無害な no-op として動き、SDK を入れた瞬間に実際のテレメトリが流れ始める。これにより「計装の追加」と「テレメトリの有効化・送信先の決定」を独立に扱える。

---

## 2. OTel の全体像(コンポーネント)

```
  ┌─ アプリ ────────────────────────────┐
  │  計装コード → [OTel API]              │
  │                 │ (実装を注入)        │
  │              [OTel SDK]               │
  │                 │ Exporter            │
  └─────────────────┼─────────────────────┘
                    │ OTLP (gRPC :4317 / HTTP :4318)
                    ▼
             [OTel Collector]   ← 受信・加工・転送
                    │ 各バックエンド固有プロトコル
        ┌───────────┼───────────┐
        ▼           ▼           ▼
   traces backend  metrics    logs backend
   (Tempo 等)      (Mimir 等)  (Loki 等)
```

アプリは OTLP で送るところまでが責務で、その先(どこへ保存するか)は Collector が引き受ける。これが [01_concepts.md](./01_concepts.md) で述べた「計装と保存先の分離」の正体である。

---

## 3. OTLP — OpenTelemetry Protocol の仕様

**OTLP** はテレメトリを運ぶための標準プロトコルだ。OTel の各 SDK と Collector はこれで会話する。

### トランスポートは2種類

| 方式 | ポート(既定) | 形式 | 本章での使用 |
|---|---|---|---|
| **gRPC** | 4317 | Protocol Buffers over HTTP/2 | Go アプリ → Collector |
| **HTTP** | 4318 | Protocol Buffers(または JSON) over HTTP/1.1 | ブラウザ → Collector |

どちらもペイロードは **Protocol Buffers** でシリアライズされる(HTTP は JSON も選べる)。本章ではサーバ間は効率の良い gRPC、ブラウザからは CORS と相性の良い HTTP を使う。

> **gRPC サーバーはどこに立つのか** — ここでよく誤解されるが、**OTLP の gRPC サーバーは OTel Collector** であり、アプリではない。アプリは `otlptracegrpc` などを使う gRPC の**クライアント**として `otel-collector:4317` にダイヤルしてテレメトリを送るだけだ([03_otel_sdk_go.md](./03_otel_sdk_go.md))。Collector 側の `receivers.otlp.protocols.grpc`(`0.0.0.0:4317`)が受信用の gRPC サーバーを立てている([08_collector.md](./08_collector.md))。本章のアプリ自身は、API を提供する **HTTP サーバー(:9100)** だけを起動し、gRPC サーバーは持たない。06_microservie の各サービスはアプリ本体が gRPC サーバー(:50051 等)だったが、本章で gRPC が登場するのは「テレメトリのエクスポート経路」だけ、という違いに注意する。

### HTTP のエンドポイント

HTTP/OTLP はシグナルごとに固定パスを持つ。

```
POST http://<collector>:4318/v1/traces
POST http://<collector>:4318/v1/metrics
POST http://<collector>:4318/v1/logs
```

本章のブラウザ計装([04_traces_e2e.md](./04_traces_e2e.md))は `OTLPTraceExporter({ url: "http://localhost:4320/v1/traces" })` でこの `/v1/traces` を叩いている(4320 はホスト公開ポート、コンテナ内 4318)。

### リクエストの中身と「部分成功」

各リクエストは `ExportTraceServiceRequest` のような protobuf メッセージで、後述のデータモデル(ResourceSpans の配列など)を運ぶ。レスポンスは **部分成功(partial success)** を返せる仕様で、「1000件中3件だけ拒否した」といった応答が可能だ。エクスポータはバッチで送り、失敗時はリトライする。

### 接続オプション

OTLP exporter は共通して `endpoint` / TLS / 圧縮 / ヘッダ を設定できる。本章はローカル開発のため平文(`WithInsecure()` 相当)で送っている(03_otel_sdk_go.md の `otlptracegrpc.New(..., WithInsecure())`)。本番ではここに TLS と認証ヘッダが入る。

---

## 4. テレメトリのデータモデル

OTLP で運ばれるデータは、3シグナルで共通の入れ子構造を持つ。最上位が **Resource**、その下が **InstrumentationScope**、さらにその下に各シグナル本体が並ぶ。

```
Resource(このテレメトリの発生元エンティティ)
  例: service.name=checkout-api, host.name, ...
  └─ InstrumentationScope(計装ライブラリ名/版)
       例: "checkout-api", "@opentelemetry/instrumentation-fetch"
       └─ Span / Metric / LogRecord  ← シグナル本体の配列
```

protobuf 上ではそれぞれ `ResourceSpans` / `ResourceMetrics` / `ResourceLogs` というトップレベル型になる。

### Resource

テレメトリが「どのエンティティから出たか」を示す属性集合。最重要属性が `service.name` で、これが Tempo/Mimir/Loki 上でサービスを識別するキーになる。Go では `resource.New(..., semconv.ServiceName("checkout-api"))`、JS では `resourceFromAttributes({ [ATTR_SERVICE_NAME]: "checkout-frontend" })` で設定する。

### Span(trace の構成単位)

| フィールド | 意味 |
|---|---|
| `trace_id` (16バイト) | リクエスト全体を貫く ID |
| `span_id` (8バイト) | この区間の ID |
| `parent_span_id` | 親区間の ID(根 span は空) |
| `name` | 区間名(例: `checkout.process`) |
| `start/end time` | 開始・終了時刻 |
| `attributes` | 任意のキー値(セマンティック規約に従う) |
| `events` | 区間内のタイムスタンプ付きイベント |
| `status` | Ok / Error |

`trace_id` を共有し `parent_span_id` でつながった span 群が1本のトレースになる。詳細な親子のつなぎ方は [04_traces_e2e.md](./04_traces_e2e.md) で扱う。

### Metric

メトリクスは「名前 + データ点(data point)の列」で、型によって意味が変わる。

| 型 | 用途 | 本章の例 |
|---|---|---|
| **Sum**(Counter) | 単調増加の累積(リクエスト数など) | `http.server.requests` |
| **Gauge** | 増減する瞬間値(メモリ使用量など) | — |
| **Histogram** | 値の分布(レイテンシなど) | `http.server.duration` |

各データ点は時刻・値・属性を持つ。Histogram はバケット境界ごとの度数を持ち、`histogram_quantile()` で分位点を計算できる([05_metrics_prom_mimir.md](./05_metrics_prom_mimir.md))。

### LogRecord

タイムスタンプ・重大度(severity)・本文(body)・属性に加え、**`trace_id` / `span_id`** を持てる。ここに trace 情報を載せることで、ログとトレースを相互に行き来できる([06_logs_loki.md](./06_logs_loki.md))。

---

## 5. セマンティック規約(Semantic Conventions)

データモデルは「箱」を定義するが、その中の **属性名** がバラバラでは相関もダッシュボードも壊れる。あるサービスが `http.method`、別のサービスが `httpMethod` と書けば、横断クエリが書けない。

**セマンティック規約**は、こうした属性名・値を標準辞書として定める仕様だ。

| 領域 | 規約された属性(例) |
|---|---|
| サービス | `service.name`, `service.version` |
| HTTP | `http.request.method`, `http.response.status_code`, `url.path` |
| DB | `db.system`, `db.namespace` |
| ホスト | `host.name`, `host.arch` |

本章の Go は `go.opentelemetry.io/otel/semconv/v1.26.0` を、JS は `@opentelemetry/semantic-conventions` を使う。`semconv.ServiceName(...)` のようなヘルパーは、この規約で決まったキー(`service.name`)を型安全に書くための薄いラッパーである。規約はバージョン管理されており(本章は v1.26.0 系)、版が上がると属性名が変わることがある点に注意する。

---

## 6. コンテキスト伝播(Context Propagation)の仕様

分散トレースを「1本」にするには、サービス境界を越えて **trace context を運ぶ**必要がある。これを担うのが **Propagator** と、その載せ方を定める **W3C Trace Context** 仕様だ。

- **traceparent** ヘッダ … `version-trace_id-span_id-flags` の形式で trace_id と親 span_id を運ぶ(HTTP標準)。
- **tracestate** ヘッダ … ベンダ固有の付加情報。
- **Baggage** … アプリ独自のキー値を伝播させる別仕様(本章では未使用)。

OTel SDK は `propagation.TraceContext{}`(Go)や既定の `W3CTraceContextPropagator`(JS)で、送信時に traceparent を**注入(inject)**し、受信時に**抽出(extract)**する。本章ではブラウザの fetch が traceparent を付与し、Go の otelhttp がそれを継続することで、ブラウザ→backend が1本のトレースになる。**フォーマットの詳細と実際の流れは [04_traces_e2e.md](./04_traces_e2e.md)** で扱う。

---

## まとめ / 関連 doc

**まとめ**

- OpenTelemetry は OpenTracing と OpenCensus の統合から生まれた CNCF プロジェクトで、**API / SDK / データモデル / セマンティック規約 / OTLP / Collector** という複数の仕様の集合である。
- 「仕様」と「言語別実装」が分離しており、本章の Go とブラウザ JS は同じ仕様の別実装にすぎない。
- **OTLP** は gRPC(:4317)/HTTP(:4318) で Protocol Buffers を運ぶ標準プロトコルで、HTTP は `/v1/traces` などの固定パスを持つ。
- テレメトリは **Resource → Scope → Span/Metric/LogRecord** の共通入れ子構造で、`service.name` がサービス識別の要。
- **セマンティック規約**が属性名を標準化し、**W3C Trace Context** が境界越えの伝播を可能にする。

**関連 doc:**
- [01_concepts.md](./01_concepts.md) — 観測性の概念と3本柱
- [03_otel_sdk_go.md](./03_otel_sdk_go.md) — この仕様を Go SDK でどう呼ぶか
- [04_traces_e2e.md](./04_traces_e2e.md) — traceparent と一貫トレースの実際
- [08_collector.md](./08_collector.md) — OTLP を受ける Collector の設定
