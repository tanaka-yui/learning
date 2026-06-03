# 09_oss_landscape: OSS 観測性ツールのランドスケープ

## 本章の選択: Grafana LGTM スタック

本章では **Loki + Grafana + Tempo + Mimir** (LGTM) を採用した。Grafana Labs がメンテナンスするコンポーネント群で、OTel Collector と組み合わせることで Metrics / Traces / Logs の3本柱をカバーする。各コンポーネントを独立してデプロイ・スケールできる柔軟性が特徴だ。

LGTM はあくまで1つの選択肢であり、目的・規模・チームの技術背景によって代替ツールが存在する。以下に主要な選択肢を整理する。

---

## Grafana Alloy — 次世代収集エージェント

**Grafana Alloy** は OTel Collector 互換の次世代テレメトリコレクターだ。HCL ライクな設定言語 (River) で宣言的にパイプラインを記述できる。Prometheus の scrape、OTel の OTLP 受信、ログの tail など複数の収集方式を1エージェントに統合できる。

**いつ選ぶか**: Prometheus Agent・OTel Collector・Promtail を別々に動かしている環境を1エージェントに統合したいとき。新規構築で Grafana スタックを前提とする場合のデフォルト候補。

---

## SigNoz / OpenObserve / Uptrace — オールインワン統合観測

これらは **Metrics / Traces / Logs を1つのサービスで統合管理する**アプローチを取る。

| ツール | バックエンド | 特徴 |
|---|---|---|
| **SigNoz** | ClickHouse | OTel ネイティブ設計。セルフホスト向け Datadog 代替を謳う |
| **OpenObserve** | Rust 製 + 独自 | 非常に軽量。S3 互換ストレージへのダイレクト保存 |
| **Uptrace** | ClickHouse | SigNoz と同系統。UI がシンプルで学習コスト低 |

**いつ選ぶか**: LGTM のような複数コンポーネント管理が煩雑だと感じるチーム、または Datadog/New Relic からの移行でオールインワン UI を求める場合。ClickHouse の圧縮効率が高いため大量ログの保存コストを下げたいケースにも有効。

---

## VictoriaMetrics — 軽量 TSDB

**VictoriaMetrics** は Prometheus 互換の時系列データベースだ。単一バイナリで動作し、Prometheus の 1/5 〜 1/10 のメモリ・ディスク使用量を実測で達成している事例が多い。Prometheus の `remote_write` を受け取れるため、本章の Mimir と同じ位置に差し替え可能だ。

**いつ選ぶか**: Mimir はスケール機能が豊富だが運用複雑度も高い。小〜中規模でシンプルさを優先する場合に VictoriaMetrics が有力な代替。既存の Prometheus インフラをほぼ変えずに長期保存・スケールを追加できる。

---

## Grafana Pyroscope — 継続プロファイリング

**Grafana Pyroscope** は **継続的プロファイリング** を提供する。CPU / メモリ / goroutine のフレームグラフを時系列で記録し、「いつ・どの関数が CPU を食ったか」を探れる。Metrics / Traces / Logs に続く **第4の柱** と呼ばれる。

Grafana 11 以降は Tempo との連携機能があり、トレースの span から「この span 実行中のフレームグラフ」を直接参照できる。

**いつ選ぶか**: パフォーマンス最適化で「メトリクス的に CPU 上昇は分かるが関数レベルでの原因が不明」という壁に当たったとき。Kubernetes 環境では eBPF 計装で言語に依存しないプロファイリングも可能。

---

## Grafana Beyla / OpenTelemetry eBPF — コード変更なし自動計装

**eBPF (extended Berkeley Packet Filter)** を使うと、アプリケーションコードを変更せずに Linux カーネルレベルでテレメトリを収集できる。

- **Grafana Beyla**: HTTP・gRPC・SQL のトレースとメトリクスを eBPF で自動収集し OTLP で出力する。Go / Python / Node.js / Java 等の主要言語に対応。
- **OpenTelemetry eBPF**: OTel プロジェクトの eBPF インストルメンテーション。カーネルプローブを使いシステムコールレベルで計装する。

**いつ選ぶか**: 計装コードを追加できないサードパーティアプリ・レガシーシステム、または計装の一貫性をインフラ側で保証したいプラットフォームチームに適す。本章のように OTel SDK で自前計装する場合との組み合わせも有効(SDK の細粒度 span + eBPF のアンビエントメトリクス)。

---

## LGTM スタックの運用上の注意点

LGTM を本番で使う場合に意識すべき点をまとめる。

### コンポーネントのバージョン管理

Grafana / Loki / Tempo / Mimir はそれぞれ独立してリリースされる。本章は collector contrib `0.110.0`、Tempo `2.6.0`、Loki `3.2.0`、Mimir `2.13.0`、Grafana `11.2.0` で動作確認しているが、バージョンの組み合わせによってデータソース provisioning の設定キーや API の互換性が変わることがある。升アップグレードは1コンポーネントずつ行い、各バックエンドの CHANGELOG を参照するのがよい。

### ストレージ設計

本章の各バックエンドはローカル filesystem を使っているが、本番では以下を検討する。

| コンポーネント | 推奨ストレージ |
|---|---|
| Tempo | S3 / GCS / Azure Blob |
| Loki | S3 / GCS + index キャッシュ |
| Mimir | S3 / GCS (object storage) |

Mimir monolithic モードは開発・小規模向けだ。本番大規模では microservices モード(ingester / querier / distributor を分離)に移行し、各コンポーネントを個別にスケールする。

### Grafana アクセス制御

本章は anonymous Admin で動作する。本番では以下を設定する。

- `auth.anonymous.enabled = false` でログインを必須にする
- OIDC / LDAP / OAuth2 でシングルサインオンを設定する
- Organization と Team でダッシュボードの閲覧権限を分ける

---

## 比較表

| ツール | 導入容易さ | UI 統合 | スケール | OTel ネイティブ度 |
|---|---|---|---|---|
| LGTM (本章) | 中(4コンポーネント) | 高(Grafana 統合) | 高(各コンポーネント独立) | 高 |
| Grafana Alloy | 高(1バイナリ) | — (収集エージェント) | 中 | 高 |
| SigNoz | 中(Docker Compose) | 高(統合 UI) | 中 | 高 |
| OpenObserve | 高(単一バイナリ) | 高(統合 UI) | 中〜高 | 高 |
| VictoriaMetrics | 高(単一バイナリ) | 中(外部 Grafana) | 高 | 中(Prom 互換) |
| Grafana Pyroscope | 高(Grafana 統合) | 高 | 中 | 中(独自SDK+OTel) |
| Beyla / eBPF | 高(Daemonset) | 中(Grafana) | 高 | 高 |

---

## まとめ / 関連 doc

- LGTM はコンポーネントの独立性とスケーラビリティが強みだが、複数コンポーネントの運用コストが伴う。
- シンプルさを優先するなら SigNoz・OpenObserve・VictoriaMetrics などオールインワン・軽量代替が選択肢になる。
- Pyroscope(継続プロファイリング)と eBPF 計装は3本柱を補完する次の学習ステップだ。

**関連 doc:**
- [01_concepts.md](./01_concepts.md) — 観測性の概念と OTel の立ち位置
- [08_collector.md](./08_collector.md) — OTel Collector の設定(Alloy との比較の出発点)
- [05_metrics_prom_mimir.md](./05_metrics_prom_mimir.md) — Mimir と Thanos・VictoriaMetrics の比較
