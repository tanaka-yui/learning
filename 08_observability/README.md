# 08_observability: OpenTelemetry 観測性ハンズオン

Metrics / Traces / Logs の3本柱を1つの Go サービス + React フロントエンドで同時に計装し、OTel Collector → Mimir / Tempo / Loki → Grafana というスタックで相関調査できる状態を体験する章だ。「アラートで気づく → トレースで経路を絞る → ログで詳細を読む」という観測性の調査フローを手を動かして習得することが狙いだ。

## 学習動線

1. [01_concepts.md](docs/01_concepts.md) — 監視と観測性の違い、3本柱のデータモデル、OTel の立ち位置
2. [02_otel_and_otlp.md](docs/02_otel_and_otlp.md) — そもそも OTel とは・OTLP・データモデル・セマンティック規約・context 伝播の仕様
3. [03_otel_sdk_go.md](docs/03_otel_sdk_go.md) — Go SDK の3プロバイダ初期化・RED ミドルウェア・otelslog ブリッジ
4. [04_traces_e2e.md](docs/04_traces_e2e.md) — W3C traceparent の仕組み・ブラウザ→Go の一貫トレース
5. [05_metrics_prom_mimir.md](docs/05_metrics_prom_mimir.md) — RED/USE・Counter/Histogram・Prometheus と Mimir の役割分担・PromQL
6. [06_logs_loki.md](docs/06_logs_loki.md) — 構造化ログ・slog+otelslog・Loki ラベル設計・LogQL
7. [07_grafana_correlation.md](docs/07_grafana_correlation.md) — データソース provisioning・相関設定・RED ダッシュボード
8. [08_collector.md](docs/08_collector.md) — Collector のパイプライン設定・agent vs gateway
9. [09_oss_landscape.md](docs/09_oss_landscape.md) — Grafana LGTM の総括・代替 OSS ツールの比較
10. [10_alerting_servicegraph.md](docs/10_alerting_servicegraph.md) — Grafana アラートメール(Mailpit)・サービスグラフ・Traces Drilldown

## クイックスタート

```bash
# スタック起動
make up

# ブラウザでフロントエンドを開いてボタンを押す
open http://localhost:5174

# Grafana でシグナルを確認 (anonymous、ログイン不要)
open http://localhost:3001

# 負荷をかけてメトリクス・ログ・トレースを溜める
make load

# 単発デモ (curl で checkout API を1回叩く)
make demo

# エラー率を上げてエラー挙動を確認 (FLAKE_RATE=0〜1)
# 既定は FLAKE_RATE=0.0 なので checkout は常に成功し、Error rate は「No data」になる。
# 502 を発生させて Error rate を立ち上げたいときに設定する。アプリだけ作り直すなら:
#   FLAKE_RATE=0.8 docker compose up -d --no-deps app
FLAKE_RATE=0.8 make up

# レイテンシアラートのデモ (p95 を押し上げる)
LATENCY_MS=500 docker compose up -d --no-deps app
# 受信したアラートメールを確認
open http://localhost:8025

# Go テスト
make test

# スタック停止・クリーンアップ
make down
```

## アクセス先一覧

| コンポーネント | URL | 用途 |
|---|---|---|
| Frontend (Vite) | http://localhost:5174 | React UI・チェックアウトボタン |
| Grafana | http://localhost:3001 | 統合ダッシュボード・Explore (anonymous Admin) |
| Prometheus | http://localhost:9090 | メトリクス確認・TargetsUI |
| Go API | http://localhost:9100 | チェックアウト API 本体 |
| Mimir | http://localhost:9009 | 長期メトリクスストレージ (内部主体) |
| Tempo | http://localhost:3200 | トレースストレージ (内部主体) |
| Loki | http://localhost:3100 | ログストレージ (内部主体) |
| Collector gRPC | http://localhost:4319 | OTLP/gRPC 受け口 (内部主体) |
| Collector HTTP | http://localhost:4320 | OTLP/HTTP 受け口 (内部主体) |
| Collector Prom | http://localhost:8889 | Prometheus scrape エンドポイント |
| Mailpit | http://localhost:8025 | アラートメールの受信トレイ(開発用 SMTP) |

## Grafana のダッシュボードとデータソース

provisioning で以下を自動登録する(`infra/grafana/provisioning/`)。Grafana の「Dashboards」「Connections → Data sources」から確認できる。

**データソース**

| 名前 | 種別 | 用途 |
|---|---|---|
| Mimir | prometheus | 長期メトリクス(アプリ + スタック)。既定データソース |
| Prometheus | prometheus | scrape 側を直接参照。各コンポーネントの死活/運用メトリクス向け |
| Tempo | tempo | トレース。logs/metrics への相関リンク付き |
| Loki | loki | ログ。trace_id でトレースへジャンプ |

**ダッシュボード**

| 名前 | 内容 |
|---|---|
| RED — checkout-api | アプリの Rate / Error rate / p95 latency([05_metrics_prom_mimir.md](docs/05_metrics_prom_mimir.md)) |
| Stack overview — observability components | スタック各コンポーネントの死活(`up`)・ヒープ使用量・取り込みサンプル率 |
| Logs — checkout-api | ログを service/level/trace_id で絞り込み |

Prometheus は **アプリの RED メトリクス(collector:8889)に加え、prometheus / mimir / tempo / loki / grafana 自身の `/metrics`** も scrape する(`infra/prometheus/prometheus.yml`)。そのため「Prometheus → Targets」で6ターゲットが UP になり、Stack overview ダッシュボードで各コンポーネントの状態を一覧できる。これらのメトリクスは `remote_write` で Mimir にも入るため、Mimir データソースからでも同じ値を引ける。

## アーキテクチャ

```
[React+Vite ブラウザ :5174]
  ├─ traces (web sdk) ─OTLP/HTTP:4320┐
  └─ fetch (traceparent付与) ─────────►[Go API :9100]
                                         ├─ traces  ─OTLP/gRPC:4319┐
                                         ├─ metrics (RED) ──────────┤
                                         └─ slog+trace_id (logs) ───┤
                                                                     ▼
                                                         [OTel Collector]
                                 traces→Tempo┐  metrics→:8889┐  logs→Loki┐
                                             │      ▲ scrape  │           │
                                             │  [Prometheus]──►[Mimir]    │
                                             │  remote_write              │
                                             ▼         ▼                  ▼
                                                   [Grafana :3001]
```

## 主要メトリクスと確認クエリ

| メトリクス名 | 型 | 説明 |
|---|---|---|
| `http_server_requests_total` | Counter | リクエスト総数 |
| `http_server_errors_total` | Counter | エラー総数 |
| `http_server_duration_milliseconds_bucket` | Histogram | レイテンシ分布 |

```promql
# Prometheus / Mimir (Grafana Explore)
sum(http_server_requests_total)
sum(http_server_errors_total)
histogram_quantile(0.95, sum(rate(http_server_duration_milliseconds_bucket[5m])) by (le))
```

```logql
# Loki (Grafana Explore)
{service_name="checkout-api"} | json
```

Mimir への直接クエリ:

```bash
curl -s -H 'X-Scope-OrgID: anonymous' \
  'http://localhost:9009/prometheus/api/v1/query?query=sum(http_server_requests_total)'
```

Loki への直接クエリ:

```bash
curl -s -G 'http://localhost:3100/loki/api/v1/query_range' \
  --data-urlencode 'query={service_name="checkout-api"}'
```

## 既知の制約

- **Exemplar**: Grafana UI 上でのメトリクス→トレースジャンプ体験を想定している。`enable_open_metrics: true` と `exemplarTraceIdDestinations` は設定済みだが、API での自動検証はしておらず、環境によっては Exemplar が表示されないことがある。
- **メトリクス反映遅延**: アプリの export 間隔は `OTEL_METRIC_EXPORT_INTERVAL=10000`(10秒)。Prometheus の scrape 間隔も 5 秒のため、起動直後はメトリクスが Grafana に届くまで最大 ~15 秒の遅延がある。
- **Error rate が「No data」になる**: `http_server_errors_total` は status>=500 が起きたときに初めて作られるカウンタなので、`FLAKE_RATE=0.0`(既定)だとエラーが0件で系列が存在せず、`rate()` の結果が空 → パネルが「No data」になる(0 ではなくデータなし)。これは「カウンタ系列は最初の観測まで存在しない」という観測の基本挙動。ダッシュボードでは `... or vector(0)` を付けて 0 表示にしてあるが、実際にエラーを見たいときは上記の `FLAKE_RATE` を上げる。
- **アラート発火の遅延**: `for:1m`(継続評価1分)+ 評価間隔30s + メトリクス反映(~10〜15秒)の組み合わせにより、ノブ(`FLAKE_RATE` / `LATENCY_MS`)を上げてからアラートメールが Mailpit に届くまで概ね1〜2分かかる。
- **Traces Drilldown プラグイン**: `grafana-exploretraces-app` は Grafana 起動時にオンラインで取得される。オフライン環境では初回取得ができないため、Traces Drilldown のメニューが表示されない。
- **サービスグラフの初期表示遅延**: Tempo metrics-generator が生成する `traces_service_graph_*` メトリクスが Mimir に書き込まれ、Grafana がクエリできる状態になるまで数十秒かかる。スタック起動直後は Service Graph タブが空のことがある。

## 環境注意

- **ポート**: 本章は 3001, 3100, 3200, 4319, 4320, 5174, 8889, 9009, 9090, 9100 を使用する。他章との非衝突を確認すること。
- **Docker リソース**: 8コンテナ(app / frontend / collector / prometheus / mimir / tempo / loki / grafana)を同時起動する。メモリ 4GB 以上を推奨。
- **イメージタグ依存**: collector contrib `0.110.0`、Tempo `2.6.0`、Loki `3.2.0`、Mimir `2.13.0`、Prometheus `v2.54.1`、Grafana `11.2.0` で動作確認済み。タグを変更すると設定の互換性が壊れることがある。
