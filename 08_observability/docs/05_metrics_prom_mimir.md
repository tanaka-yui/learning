# 05_metrics_prom_mimir: メトリクス・Prometheus・Mimir

## メトリクスの種類: Counter と Histogram

OTel SDK が扱うメトリクス型のうち、RED パターンで中心的に使うのは **Counter** と **Histogram** だ。

| 型 | 意味 | 本章での用途 |
|---|---|---|
| **Counter** | 単調増加する累積値 | リクエスト数・エラー数 |
| **Histogram** | 値の分布をバケットで集計 | レイテンシ分布・p95/p99 算出 |
| Gauge | 任意の瞬間値 | メモリ使用量・接続数 |
| UpDownCounter | 増減する累積値 | キューの深さ |

Counter は「合計が何件か」だけを答えるが、Histogram は「どの区間に何件分布するか」をバケット別に記録するため、パーセンタイルレイテンシの計算に使える。

---

## RED と USE — 2つの監視フレームワーク

### RED (サービス視点)

マイクロサービスのエンドポイント監視に適す。「このサービスはユーザに正しく応答しているか」を問う。

| シグナル | 意味 | 本章のメトリクス |
|---|---|---|
| **R**ate | 1秒あたりのリクエスト数 | `http_server_requests_total` |
| **E**rrors | エラー率 (5xx 等) | `http_server_errors_total` |
| **D**uration | レイテンシ分布 (p95/p99) | `http_server_duration_milliseconds_*` |

### USE (リソース視点)

インフラ・ミドルウェア層の監視に適す。「このリソースは飽和していないか」を問う。

| シグナル | 意味 | 例 |
|---|---|---|
| **U**tilization | 使用率 | CPU 使用率・ディスク使用率 |
| **S**aturation | 飽和度 | キュー長・スレッドプール待ち |
| **E**rrors | エラー数 | TCP 再送・I/O エラー |

本章の RED ダッシュボードはサービス層の監視だ。インフラ層には USE を補完として使う。

---

## 本章のメトリクス経路

```
[Go API]
  └─ OTel Meter (PeriodicReader, 10s間隔)
       └─ OTLP/gRPC → [OTel Collector]
                           └─ prometheus exporter :8889
                                  ▲ scrape (5s間隔)
                           [Prometheus :9090]
                                  └─ remote_write
                                       └─ [Mimir :9009]
                                              ▲ query
                                       [Grafana :3001]
```

### 実際のメトリクス名 (OTel → Prometheus 変換後)

OTel SDK の命名規則(ドット区切り)は Prometheus が認識できる形式(アンダースコア区切り + 型サフィックス)に Collector が変換する。

| OTel 名 | Prometheus 名 | 型 |
|---|---|---|
| `http.server.requests` | `http_server_requests_total` | Counter |
| `http.server.errors` | `http_server_errors_total` | Counter |
| `http.server.duration` | `http_server_duration_milliseconds_bucket` | Histogram |
| `http.server.duration` | `http_server_duration_milliseconds_sum` | Histogram |
| `http.server.duration` | `http_server_duration_milliseconds_count` | Histogram |

ラベル: `http_route`, `http_status_code`, `job`

---

## Prometheus と Mimir を併用する理由

Prometheus はメトリクスの **スクレイプ** と **短期ローカル保存** に特化した設計だ。単一プロセスで完結するため手軽だが、以下の制約がある。

- **単一ノード**: 水平スケールや HA(高可用性)が標準では難しい。
- **保存期間**: デフォルトは 15 日。長期保存は別途設計が必要。
- **マルチテナント**: 複数チームのメトリクスを分離する機能がない。

**Mimir** は Grafana Labs が開発する長期・スケーラブルなメトリクスストレージだ(Cortex の後継)。`remote_write` という Prometheus 標準プロトコルでデータを受け取り、長期保存・水平スケール・マルチテナントを担う。

現場では「**Prometheus で収集 → remote_write で Mimir へ転送**」という役割分担が一般的だ。Prometheus の軽量スクレイプ機能はそのまま活かし、長期保管と大規模クエリは Mimir に委ねる。

本章では Mimir を monolithic モード(`-target=all`) の単一コンテナで動かし、長期保存・スケールの恩恵を軽量に体験できる構成にしている。

---

## remote_write とは

`remote_write` は Prometheus が収集したメトリクスをリモートのエンドポイントに継続的に転送するプロトコルだ。

```yaml
# Prometheus の設定 (抜粋)
remote_write:
  - url: "http://mimir:9009/api/v1/push"
    headers:
      X-Scope-OrgID: anonymous
```

`X-Scope-OrgID` ヘッダは Mimir のテナント識別子だ。本章はマルチテナントを無効化しているが、ヘッダ自体は必須のため `anonymous` を固定値として送る。

Mimir への直接クエリは以下で確認できる。

```bash
curl -s \
  -H 'X-Scope-OrgID: anonymous' \
  'http://localhost:9009/prometheus/api/v1/query?query=sum(http_server_requests_total)'
```

---

## Thanos との比較

**Thanos** は Prometheus の既存 TSDB を sidecar 経由で読み込み、オブジェクトストレージに保存する設計だ。既存の Prometheus インフラを変えずに長期保存を追加できる点が強みだが、コンポーネント数(sidecar・store・query・compactor)が多く運用複雑度が上がる。

**Mimir** は remote_write を受け取るサーバ型設計のため、Prometheus 側の変更が最小(remote_write URL の追記のみ)で済む。新規構築では Mimir を選ぶケースが増えており、本章もこの方針だ。

---

## PromQL 例

```promql
# リクエスト率 (合計)
sum(http_server_requests_total)

# エラー数
sum(http_server_errors_total)

# p95 レイテンシ (直近5分)
histogram_quantile(
  0.95,
  sum(rate(http_server_duration_milliseconds_bucket[5m])) by (le)
)

# ルート別エラー率
sum(rate(http_server_errors_total[1m])) by (http_route)
  /
sum(rate(http_server_requests_total[1m])) by (http_route)
```

Grafana の Explore でデータソースを Mimir に切り替えてそのまま実行できる。

### 「No data」と 0 の違い — カウンタ系列の生成タイミング

`http_server_errors_total` のような OTel カウンタは、**最初にそのラベル組で観測(`Add`)されるまで系列が存在しない**。本章のミドルウェアはエラー(status>=500)のときだけ `errs.Add(...)` するため、`FLAKE_RATE=0.0`(既定)でエラーが0件のあいだは系列が作られず、`sum(rate(http_server_errors_total[1m]))` の結果は**空**になる。Grafana のパネルはこれを「**No data**」と表示する(値が 0 なのではなく、データが存在しない)。

これは観測の基本的な落とし穴だ。「0 と表示したい」場合は PromQL 側で空ベクトルを 0 で埋める:

```promql
sum(rate(http_server_errors_total[1m])) or vector(0)
```

本章の RED ダッシュボード(`red.json`)の Error rate パネルはこの形にしてある。実際にエラーを発生させてグラフを立ち上げたいときは、`FLAKE_RATE` を上げてアプリを再起動する(`FLAKE_RATE=0.8 docker compose up -d --no-deps app`)。詳細は [01_concepts.md](./01_concepts.md) の3本柱と本章 README の「既知の制約」を参照。

---

## Exemplar — メトリクスからトレースへジャンプ

Exemplar は **メトリクスの特定サンプルに trace_id を紐付けるアノテーション**だ。Grafana のメトリクスグラフ上で Exemplar のドット(◆)をクリックすると、対応する Tempo のトレースに直接ジャンプできる。

有効化には以下の設定が揃う必要がある。

1. Collector の `enable_open_metrics: true`(Prometheus exporter)
2. Prometheus の Exemplar 受け取り設定
3. Grafana データソースの `exemplarTraceIdDestinations` 設定

> **既知の制約**: exemplars は Grafana UI 上での体験を想定しており、API での自動検証はしていない。`enable_open_metrics` と `exemplarTraceIdDestinations` は設定済みだが、環境によっては Exemplar が表示されないことがある。

---

## まとめ / 関連 doc

- RED(Rate / Errors / Duration)はサービス層の健全性を測り、USE はリソース層の飽和を測る。
- Counter は累積数、Histogram は分布をバケットで記録しパーセンタイル算出を可能にする。
- Prometheus が軽量スクレイプを担い、Mimir が remote_write で長期・スケーラブルな保存を担う役割分担が一般的だ。
- Exemplar によりメトリクスのピーク時点から直接トレースに飛べるが、環境により非表示になることがある。

**関連 doc:**
- [01_concepts.md](./01_concepts.md) — メトリクスの概念と全体スタック
- [03_otel_sdk_go.md](./03_otel_sdk_go.md) — OTel Meter の初期化と RED ミドルウェア
- [08_collector.md](./08_collector.md) — Collector の prometheus exporter 設定
- [07_grafana_correlation.md](./07_grafana_correlation.md) — Mimir データソースと exemplarTraceIdDestinations
