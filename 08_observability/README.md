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
FLAKE_RATE=0.8 make up

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

## 環境注意

- **ポート**: 本章は 3001, 3100, 3200, 4319, 4320, 5174, 8889, 9009, 9090, 9100 を使用する。他章との非衝突を確認すること。
- **Docker リソース**: 8コンテナ(app / frontend / collector / prometheus / mimir / tempo / loki / grafana)を同時起動する。メモリ 4GB 以上を推奨。
- **イメージタグ依存**: collector contrib `0.110.0`、Tempo `2.6.0`、Loki `3.2.0`、Mimir `2.13.0`、Prometheus `v2.54.1`、Grafana `11.2.0` で動作確認済み。タグを変更すると設定の互換性が壊れることがある。
