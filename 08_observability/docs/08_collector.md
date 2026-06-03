# 08_collector: OTel Collector の設定とパイプライン

## Collector の役割

OTel Collector はアプリケーション(計装)とバックエンド(保存先)の間に立つ **中継プロセス**だ。アプリは OTLP という単一プロトコルで Collector にテレメトリを送るだけでよく、どのバックエンドに保存するかは Collector の設定で制御できる。

```
アプリ(OTel SDK) ──OTLP──► Collector ──► Tempo / Mimir / Loki / ...
```

この分離により以下のメリットが生まれる。

- バックエンドを差し替えてもアプリコードは変更不要
- バッファリング・リトライ・サンプリングを Collector に集約できる
- ログ/メトリクス/トレースの3シグナルを1エンドポイントで受け付けられる

---

## receivers / processors / exporters / pipelines の構造

```yaml
receivers:   # テレメトリの受け口
processors:  # 変換・バッファリング
exporters:   # バックエンドへの送信
service:
  pipelines: # receivers → processors → exporters をつなぐ
```

本章の `infra/otel-collector/config.yaml` はこの4セクションで構成される。

---

## 本章の設定詳細

### receivers

```yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317       # Go API からの OTLP/gRPC
      http:
        endpoint: 0.0.0.0:4318       # ブラウザからの OTLP/HTTP
        cors:
          allowed_origins:
            - "http://localhost:5174" # Vite dev server のオリジン
```

gRPC(4317) は Go API が使用し、HTTP(4318) はブラウザが使用する。CORS の `allowed_origins` はブラウザの preflight を通過させるために必須だ。ホスト側では `4319:4317`、`4320:4318` にマッピングされている。

### processors

```yaml
processors:
  batch:
    timeout: 1s
    send_batch_size: 512
```

`batch` processor はテレメトリをまとめてからエクスポートする。個々の span やメトリクスポイントが到着するたびに送信するのではなく、**最大 512 件** または **1 秒経過** のいずれか早い方でまとめて送信する。これによりネットワーク往復回数と接続オーバーヘッドを削減できる。

### exporters

```yaml
exporters:
  otlp/tempo:                         # トレース → Tempo
    endpoint: tempo:4317
    tls:
      insecure: true

  prometheus:                          # メトリクス → Prometheus がスクレイプ
    endpoint: 0.0.0.0:8889
    enable_open_metrics: true          # Exemplar 対応

  otlphttp/loki:                       # ログ → Loki
    endpoint: http://loki:3100/otlp

  debug:
    verbosity: basic                   # コンソール出力(開発用)
```

複数の exporter を同じプロトコル(`otlp`)で使う場合は `otlp/tempo` のようにスラッシュ区切りでエイリアスを付ける。

#### `enable_open_metrics: true` と Exemplar

`prometheus` exporter に `enable_open_metrics: true` を設定すると、OpenMetrics 形式でメトリクスを公開する。OpenMetrics は **Exemplar** (メトリクスポイントに trace_id を紐付けるアノテーション) をサポートしており、Prometheus が Exemplar ごとスクレイプすることで、Grafana のメトリクスグラフから対応するトレースに直接ジャンプできるようになる。

### pipelines

```yaml
service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [batch]
      exporters: [otlp/tempo, debug]   # Tempo + コンソール
    metrics:
      receivers: [otlp]
      processors: [batch]
      exporters: [prometheus]          # Prometheus がスクレイプする形式で公開
    logs:
      receivers: [otlp]
      processors: [batch]
      exporters: [otlphttp/loki]       # Loki へ OTLP/HTTP
```

3シグナルそれぞれに独立したパイプラインが存在し、receiver を共有しながら exporter を分岐させる構造だ。

---

## メトリクスのフロー: Collector → Prometheus → Mimir

```
Collector (prometheus exporter :8889)
    ▲
    │ scrape
[Prometheus] ──remote_write──► [Mimir]
```

`prometheus` exporter は Prometheus が **スクレイプ** するエンドポイントを公開する。Prometheus は定期的に `:8889/metrics` を叩いてメトリクスを取得し、Mimir に remote_write で転送する。Grafana はデータソースとして Mimir を参照する。

OTel のメトリクス名 (`http.server.requests`) はドットをアンダースコアに変換し、型に応じたサフィックスが付く。

| OTel 名 | Prometheus 名 |
|---|---|
| `http.server.requests` | `http_server_requests_total` |
| `http.server.errors` | `http_server_errors_total` |
| `http.server.duration` | `http_server_duration_milliseconds_{bucket,sum,count}` |

---

## agent デプロイ vs gateway デプロイ

| デプロイ形態 | 説明 | 用途 |
|---|---|---|
| **agent** | 各ホスト/Pod にサイドカーとして配置 | ホスト固有のメタデータ付与、ローカルバッファリング |
| **gateway** | 複数サービスからテレメトリを集約する中央 Collector | フィルタリング・サンプリング・バックエンドへの集約送信 |

本章は単一のゲートウェイ構成だ。本番では agent がホストレベルのメトリクスを収集し、gateway が各 agent からデータを受け取ってバックエンドに送る2段階構成が一般的だ。

---

## なぜ全シグナルを1つの Collector に集約するか

1. **オペレーション簡素化**: アプリが送信先エンドポイントを1つだけ管理すればよい。
2. **相関の保証**: Collector 内で trace_id がログとメトリクスの両方に揃っているため、Exemplar 付与などの相関処理を1か所で実装できる。
3. **バックエンド差し替えの容易さ**: Collector の exporter 設定を変えるだけでバックエンドを切り替えられる。アプリの再デプロイは不要だ。
4. **バッファリング**: アプリが直接バックエンドに送ると、バックエンドの障害がアプリに波及する。Collector がバッファとなりアプリへの影響を遮断する。

---

## まとめ / 関連 doc

- Collector は receivers → processors → exporters → pipelines の4要素で構成され、シグナルを受け取り変換してバックエンドに送る。
- `batch` processor でネットワーク往復を削減し、`enable_open_metrics: true` で Exemplar を有効化する。
- 全シグナルを1 Collector に集約することで、相関・運用・バックエンド差し替えが容易になる。

**関連 doc:**
- [01_concepts.md](./01_concepts.md) — スタック全体像とシグナルの概念
- [03_otel_sdk_go.md](./03_otel_sdk_go.md) — アプリ側の OTLP exporter 設定
- [04_traces_e2e.md](./04_traces_e2e.md) — Collector の CORS 設定がトレース連携に与える影響
