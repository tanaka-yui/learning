# 01_concepts: オブザーバビリティの基礎概念

## 監視(Monitoring)と観測性(Observability)の違い

従来の **監視** は「あらかじめ予測した障害」を検知するためのものだ。CPU使用率が 80% を超えたらアラート、といったルールベースのアプローチが典型例である。これは **既知の未知(known unknowns)**、つまり「何が壊れうるかはわかっている」問題には有効だ。

一方 **観測性(Observability)** は、システムの外部出力(テレメトリ)だけから内部状態を推論できる性質を指す。これにより **未知の未知(unknown unknowns)**、すなわち「どんな問題が起きているかすら事前にわからない」状況に対しても「なぜそうなっているのか」を問えるようになる。

> 監視は「何かが壊れている」と教えてくれる。観測性は「なぜ壊れているのか」を答えさせてくれる。

本章のスタックはその観測性を実現するための3本柱 (Metrics / Traces / Logs) を同時に扱う。

---

## 3本柱のデータモデルと使い分け

| シグナル | データモデル | 得意なこと | 粒度 | 相対コスト |
|---|---|---|---|---|
| **Metrics** | 時系列(時刻+数値+ラベル) | 傾向把握・SLO監視・アラート | 集計値(低粒度) | 低 |
| **Traces** | span のツリー(開始/終了時刻+属性) | リクエスト1本の経路追跡・ボトルネック特定 | 1リクエスト単位(高粒度) | 中〜高 |
| **Logs** | イベント(時刻+テキスト+構造化フィールド) | 詳細なコンテキスト・例外スタックトレース | 任意(行単位) | 中 |

### 各シグナルを使うべき場面

- **Metrics** → 「エラー率は上がっているか？」「レイテンシの p99 は？」
- **Traces** → 「このリクエストはどこで遅くなっているか？」「フロントエンドから DB まで何 ms かかったか？」
- **Logs** → 「エラー時のリクエストパラメータは？」「どの行で例外が出たか？」

3つを組み合わせることで「アラートで気づく → トレースで経路を絞る → ログで詳細を読む」という調査フローが成立する。

---

## OpenTelemetry の立ち位置

OpenTelemetry (OTel) は CNCF のプロジェクトで、**ベンダ中立の計装 API / SDK / プロトコル(OTLP)** を定義する。

```
アプリ(OTel SDK で計装)
   │  OTLP(gRPC or HTTP)
   ▼
OTel Collector
   │  各バックエンド固有のプロトコルに変換
   ├─► Tempo / Jaeger / Zipkin  (traces)
   ├─► Prometheus / Mimir       (metrics)
   └─► Loki / Elasticsearch     (logs)
```

重要な利点は **計装と保存先の分離** だ。アプリコードは OTel SDK の API だけを呼び出し、どのバックエンドに送るかは Collector の設定で制御できる。Tempo を Jaeger に差し替えてもアプリを一切変更しなくてよい。

OTel が「そもそも何を標準化したプロジェクトなのか」、そして OTLP・データモデル・セマンティック規約・コンテキスト伝播といった**プロトコル仕様**の中身は、次の [02_otel_and_otlp.md](./02_otel_and_otlp.md) で詳しく扱う。本 doc(01)は観測性という性質の話、02 は OTel という道具そのものの話、という位置づけだ。

---

## 本章のスタック全体像

```
[React+Vite ブラウザ]
  ├─ traces(web sdk) ─OTLP/HTTP:4318┐
  ├─ Web Vitals      ─OTLP/HTTP:4318┤
  └─ fetch(traceparent付与) ─►[Go API net/http+otelhttp]
                                ├─ traces  ─OTLP/gRPC:4317┐
                                ├─ metrics(RED) ─OTLP/gRPC┤
                                └─ slog+trace_id ─OTLP logs┤
                                                           ▼
                                                 [OTel Collector]
   traces→Tempo┐  metrics→prom exporter:8889┐  logs→Loki┐
               │         ▲scrape             │          │
               │   [Prometheus]─remote_write─►[Mimir]   │
               ▼                              ▼          ▼
                            [ Grafana ] ◄────────────────┘
```

### ポート早見表

| コンポーネント | ホスト公開ポート |
|---|---|
| Go API | 9100 |
| Frontend (Vite) | 5174 |
| Collector OTLP gRPC | 4319 → (コンテナ内 4317) |
| Collector OTLP HTTP | 4320 → (コンテナ内 4318) |
| Collector Prometheus exporter | 8889 |
| Prometheus | 9090 |
| Mimir | 9009 |
| Tempo | 3200 |
| Loki | 3100 |
| Grafana | 3001 |

---

## 本章の学習動線

| Doc | 内容 |
|---|---|
| **02_otel_and_otlp.md** | そもそも OTel とは・OTLP・データモデル・セマンティック規約・context 伝播の仕様 |
| **03_otel_sdk_go.md** | Go 側の3プロバイダ初期化・RED ミドルウェア・otelslog ブリッジ |
| **04_traces_e2e.md** | W3C traceparent の仕組み・ブラウザ→Go の一貫トレース |
| **05_metrics_prom_mimir.md** | RED メトリクス・Prometheus→remote_write→Mimir・PromQL |
| **06_logs_loki.md** | 構造化ログ・trace_id 相関・Loki/LogQL |
| **07_grafana_correlation.md** | データソース相関・3本柱の往復・RED ダッシュボード |
| **08_collector.md** | Collector の設定・パイプライン・デプロイ形態 |
| **09_oss_landscape.md** | 最近人気の観測 OSS の比較・提案 |
| **10_alerting_servicegraph.md** | Grafana アラートメール(Mailpit)・サービスグラフ(Tempo metrics-generator) |

---

## まとめ / 関連 doc

- 監視は既知の異常を検知し、観測性は未知の問題を推論可能にする。
- Metrics / Traces / Logs は互いに補完し合うシグナルであり、Grafana で相関させることで調査が完結する。
- OTel は計装と保存先を分離し、バックエンド依存を排除する。

**関連 doc:**
- [02_otel_and_otlp.md](./02_otel_and_otlp.md) — OTel とは何か・OTLP とデータモデルの仕様
- [03_otel_sdk_go.md](./03_otel_sdk_go.md) — Go SDK の具体的な初期化コード
- [04_traces_e2e.md](./04_traces_e2e.md) — ブラウザ→Go の一貫トレース
- [08_collector.md](./08_collector.md) — Collector の設定詳細
