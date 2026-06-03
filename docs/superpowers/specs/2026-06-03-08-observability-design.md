# 08_observability 設計仕様

- 日付: 2026-06-03
- 対象: 学習リポジトリ新規章 `08_observability/`
- テーマ: OpenTelemetry を軸にした観測性（Observability）の体感学習

## 1. 目的とスコープ

観測性そのものを主役にした最小構成のハンズオン章を作る。小さな Go API と React フロントエンドを題材に、OpenTelemetry で **メトリクス・トレース・ログの3本柱**を計装し、**ブラウザから backend まで一貫した分散トレース**を実現する。可視化・保存は **Grafana LGTM スタック**（Loki / Grafana / Tempo / Mimir）で行い、現場で一般的な **Prometheus → remote_write → Mimir** の役割分担を実体として体験する。

### やること
- 新規・最小の Go HTTP API 単体（`/api/checkout`、内部関数を span 分割、確率的エラー注入）
- React + Vite フロント（06 と統一）。OTel JS でブラウザ traces + Web Vitals metrics、`traceparent` 伝播
- OTel Collector を統合ハブに、traces→Tempo / metrics→Prometheus→Mimir / logs→Loki
- Grafana でデータソース相関（exemplars, trace_id によるログ⇄トレース往復）
- docs/ に番号付き解説。最後に最近人気の観測 OSS を比較・提案

### やらないこと（YAGNI）
- 06 マイクロサービス群への統合（08 は独立・最小に保つ）
- アラート（Alertmanager）、SLO、本番運用の HA 構成
- Mimir のマイクロサービス展開（monolithic 単一バイナリに限定）
- 認証・永続化・ビジネスロジックの作り込み（題材は観測が映える最小限）

## 2. アーキテクチャ / トポロジ

すべての信号を OTel Collector に集約し、Collector を統合ハブにする。アプリ側は OTLP で送るだけにして、保存・可視化バックエンドを背後で差し替え可能にする。

```
[React + Vite (ブラウザ)]
  ├─ traces (otel web sdk)  ──OTLP/HTTP :4318─┐
  ├─ Web Vitals (metrics)   ──OTLP/HTTP :4318─┤
  └─ fetch(traceparent付与) ─► [Go API  (net/http + otelhttp)]
                                  ├─ traces  ─OTLP/gRPC :4317─┐
                                  ├─ metrics (RED) ─OTLP/gRPC─┤
                                  └─ slog+trace_id ─OTLP logs─┤
                                                              ▼
                                                    [OTel Collector]
        traces→ Tempo ┐   metrics→ prometheus exporter :8889 ┐   logs→ Loki ┐
                      │                ▲ scrape               │             │
                      │           [Prometheus] ─remote_write─► [Mimir]      │
                      ▼                                        ▼            ▼
                                              [ Grafana ]  ◄───────────────┘
                          datasources: Tempo / Mimir / Loki
                          相関: trace⇄metrics(exemplars), trace⇄logs(trace_id)
```

### 信号ごとの経路
- **Traces**: ブラウザの root span → `traceparent` ヘッダ → Go の otelhttp が継続 → 内部 span（validate/reserve-stock/charge）。Collector → Tempo。Grafana 上で1本のトレースとして可視化。
- **Metrics**: アプリ → OTLP push → Collector の prometheus exporter(:8889) → **Prometheus が scrape → remote_write → Mimir** → Grafana は Mimir を参照。pull/scrape と remote_write の両方を学べる。
- **Logs**: `slog` + OTel bridge で全ログに `trace_id`/`span_id` を自動付与 → OTLP → Collector → Loki → Grafana。

### 相関（Grafana データソース設定で実現）
- metrics→trace: exemplars 経由でメトリクスのデータ点からトレースへジャンプ
- trace→logs / logs→trace: `trace_id` をキーに Tempo⇄Loki を往復

## 3. ディレクトリ構成

```
08_observability/
├── README.md                # 学習動線・クイックスタート・アーキ図・ポート一覧
├── docs/
│   ├── 01_concepts.md            # 観測性とは / 監視との違い / 3本柱 / OTelの役割・データモデル
│   ├── 02_otel_sdk_go.md         # Go計装: Tracer/Meter/Logger Provider, otelhttp, slog bridge, resource/semconv
│   ├── 03_traces_e2e.md          # 分散トレース / span / context propagation / W3C traceparent / フロント→backend
│   ├── 04_metrics_prom_mimir.md  # RED / OTLP→Collector→Prometheus scrape→remote_write→Mimir / PromQL / exemplars
│   ├── 05_logs_loki.md           # 構造化ログ / trace_id相関 / Loki / LogQL
│   ├── 06_grafana_correlation.md # データソース相関設定 / 3本柱の往復 / ダッシュボード
│   ├── 07_collector.md           # receivers/processors/exporters / pipeline / なぜ集約するか
│   └── 08_oss_landscape.md       # 最近人気OSSの比較・提案
├── app/                     # Go API
│   ├── go.mod
│   ├── main.go
│   ├── internal/obs/otel.go      # Tracer/Meter/Logger Provider 初期化 + propagator
│   ├── internal/checkout/...     # ハンドラ + validate/reserve/charge + 確率エラー注入
│   ├── Dockerfile
│   └── *_test.go
├── frontend/                # React + Vite
│   ├── package.json
│   ├── src/otel.ts               # web SDK 初期化, fetch instrumentation, web-vitals
│   ├── src/App.tsx
│   └── Dockerfile.dev
├── infra/
│   ├── otel-collector/config.yaml
│   ├── prometheus/prometheus.yml      # scrape collector :8889 + remote_write → mimir
│   ├── mimir/mimir.yaml               # monolithic (-target=all)
│   ├── tempo/tempo.yaml
│   ├── loki/loki.yaml
│   └── grafana/provisioning/          # datasources(相関込み) + dashboards
├── docker-compose.yml       # app, frontend, collector, prometheus, mimir, tempo, loki, grafana
└── Makefile                 # make up / demo / load / down
```

go.work への登録は他章の慣習に合わせる（章内に独立 go.work を置くか、`08_observability/app` を単独モジュールとして扱う。実装計画時に既存章の流儀を再確認して決定）。

## 4. コンポーネント設計

### 4.1 Go API (`app/`)
- **エンドポイント**: `POST /api/checkout`。内部で `validate(req) → reserveStock(ctx) → charge(ctx)` を順に呼び、それぞれ子 span を張る。
- **エラー注入**: `charge` は環境変数 `FLAKE_RATE`（既定 0.0、06 踏襲）で一定確率失敗。トレースのエラー span / メトリクスのエラー率 / ログの相関を「見える」題材にする。
- **計装**:
  - traces: `otelhttp.NewHandler` でHTTPサーバ自動計装 + 内部 span 手動
  - metrics: RED（リクエスト数 Counter / エラー数 Counter / レイテンシ Histogram）。otelhttp 由来 + カスタム
  - logs: `slog` + OTel bridge で全ログに `trace_id`/`span_id` 付与
- **OTel 初期化** (`internal/obs/otel.go`): 06 の `otlptracegrpc` / `propagation.TraceContext{}` / `resource` + `semconv.ServiceName` を踏襲し、MeterProvider・LoggerProvider を追加。SDK は 06 と同系統のバージョン（otel SDK v1.43 系 / otelhttp v0.68 系）を基準に、実装時に最新安定版を Context7 で確認。
- **TDD**: ハンドラのバリデーション・エラー注入・正常系をテスト。

### 4.2 フロントエンド (`frontend/`)
- React + Vite（06 と統一）。ボタン → `fetch('/api/checkout')`。
- `src/otel.ts`: `@opentelemetry/sdk-trace-web` + fetch instrumentation で root span 生成と `traceparent` 付与、OTLP/HTTP(:4318) で Collector へ送信。CORS を Collector 側で許可。
- Web Vitals を OTLP metrics として送信。

### 4.3 OTel Collector (`infra/otel-collector/config.yaml`)
- receivers: `otlp`（grpc :4317 / http :4318）
- processors: `batch`
- exporters: `otlp/tempo`（traces）、`prometheus`（:8889、metrics をスクレイプ用に公開）、`loki`（logs）または `otlphttp` 経由
- pipelines: traces / metrics / logs の3系統

### 4.4 保存・可視化スタック
- **Prometheus**: Collector(:8889) を scrape、`remote_write` で Mimir へ転送
- **Mimir**: monolithic（`-target=all`）単一コンテナ。Grafana の主データソース（metrics）
- **Tempo**: traces 保存。exemplars / trace→logs 相関設定
- **Loki**: logs 保存。LogQL、trace_id でのフィルタ
- **Grafana**: provisioning でデータソース3種を自動登録、相関を有効化、サンプルダッシュボード（RED + トレース + ログパネル）を同梱

### 4.5 オーケストレーション
- `docker-compose.yml`: 全コンポーネントを1コマンドで起動。ポートは既存章と衝突しない帯を選ぶ（実装時に確認）。
- `Makefile`: `make up`（起動）、`make demo`（1リクエスト送信）、`make load`（負荷生成でメトリクス/トレースを蓄積）、`make down`。

## 5. ドキュメント方針

- 既存章の doc スタイル（日本語・である調・図表・コマンド例・「まとめ/関連doc」）を踏襲。
- 各 doc は「概念 → なぜ必要か → 本章のコードでどう実現しているか → 手を動かす手順」の順。
- `08_oss_landscape.md` には以下を「いつ選ぶか」付きで掲載し、採用した LGTM との位置づけを明記:
  - Grafana Alloy（新世代の収集エージェント、Collector 互換）
  - SigNoz / OpenObserve / Uptrace（オールインワン統合観測、ClickHouse/Rust 系）
  - VictoriaMetrics（Prometheus/Mimir 代替の軽量・高効率 TSDB）
  - Grafana Pyroscope（継続プロファイリング、“第4の柱”）
  - Grafana Beyla / OpenTelemetry eBPF（コード変更なしの自動計装）

## 6. 成功条件（受け入れ基準）

1. `make up` で全コンポーネントが起動し、healthy になる。
2. ブラウザでボタンを押すと、Grafana(Tempo) に **ブラウザ span → Go HTTP span → 内部 span が1本のトレース**として表示される。
3. Grafana(Mimir) で RED メトリクス（リクエスト数・エラー率・レイテンシ）が PromQL で確認でき、Prometheus が remote_write で Mimir に転送していることを設定・UI で追える。
4. Grafana(Loki) でログが `trace_id` で絞り込め、トレース⇄ログを往復できる。
5. メトリクスのデータ点から exemplars 経由でトレースへジャンプできる。
6. `FLAKE_RATE` を上げるとエラー率・エラートレース・エラーログが連動して増える。
7. Go API のテストが通る（`make test` 相当、TDD）。
8. docs 8本が揃い、`08_oss_landscape.md` に OSS 比較がある。

## 7. リスク / 留意点

- **Mimir/LGTM のリソース**: コンテナ数が多い。monolithic モードと最小設定でメモリを抑える。起動順序は depends_on / healthcheck で制御。
- **ブラウザ→Collector の CORS**: Collector の OTLP/HTTP receiver で CORS 許可が必要。
- **ポート衝突**: 既存章（06: 4317/4318/16686 等）と同時起動しうるため、08 は別帯を割り当てる。
- **SDK バージョン整合**: Go/JS とも OTel は更新が速い。実装時に Context7 で最新安定版とブレーキング変更を確認。
- **学習ノイズ**: 3本柱フル + LGTM はボリュームがある。各 doc を簡潔に保ち、コードは題材最小限に絞る。
