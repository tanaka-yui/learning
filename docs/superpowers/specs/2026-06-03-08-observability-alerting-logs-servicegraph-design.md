# 08_observability 拡張 設計仕様 — アラートメール / ログ閲覧 / トレースのグラフ表示

- 日付: 2026-06-03
- 対象: 既存章 `08_observability/` への追加機能
- 前提: 本章は既に動作済み（Go API + React フロント + OTel Collector + Prometheus→Mimir / Tempo / Loki / Grafana、RED + Stack overview ダッシュボード、Prometheus/Mimir/Tempo/Loki データソース）。

## 1. 目的とスコープ

既存の観測スタックに、運用で必須の3つの体験を追加する。

1. **アラートメール**: Grafana のアラートルールが発火したら、開発用 SMTP サーバー(Mailpit)経由でメールに飛ばす。
2. **ログ閲覧 UI**: ダッシュボードに加えてログを探索できるようにする(Logs ダッシュボード + Grafana Explore)。
3. **トレースのグラフィカル追跡**: フロント→backend をノードグラフ(Service Graph / Node Graph)で可視化し、Traces Drilldown アプリでも探索できるようにする。

### やること
- Mailpit コンテナ追加、Grafana の SMTP 設定、contact point / notification policy / alert rules の provisioning
- 2本のアラートルール(エラー率・p95 レイテンシ)
- アプリへのレイテンシ注入 `LATENCY_MS`(レイテンシアラートのデモ用)
- Logs ダッシュボード(service / level / trace_id で絞り込み)+ Explore の doc 化
- Tempo の metrics-generator(service-graphs / span-metrics)有効化と Mimir への remote_write
- Tempo データソースの serviceMap / nodeGraph 設定
- Traces Drilldown アプリ(`grafana-exploretraces-app`)のインストール
- README / docs 更新

### やらないこと(YAGNI)
- 本番相当のアラート設計(SLO、多段エスカレーション、Slack/PagerDuty 連携)
- 認証付き SMTP / 実メール送信(Mailpit で完結)
- マルチテナント、アラートの sharding/HA
- ログのメトリクス化(Loki ruler)やログベースアラート

## 2. アーキテクチャ(追加分)

```
                         [ checkout-api ]
                          FLAKE_RATE → 5xx     LATENCY_MS → 遅延
                                 │ OTLP
                                 ▼
                          [OTel Collector] → metrics → Prometheus → remote_write → Mimir
                                 └ traces → Tempo ──┐
                                                    │ metrics-generator
                                                    │ (service-graphs / span-metrics)
                                                    └ remote_write → Mimir
                                 ┌──────────────────┘
                          [ Grafana ]
                            ├ Alert rules(Mimir/PromQL) ─発火→ contact point(email)
                            │                                   │ SMTP :1025
                            │                                   ▼
                            │                              [ Mailpit ]  Web 受信トレイ :8025
                            ├ Logs ダッシュボード / Explore → Loki
                            └ Service Graph / Node Graph / Traces Drilldown → Tempo(+Mimir)
```

## 3. コンポーネント設計

### 3.1 Mailpit(開発用 SMTP)
- `docker-compose.yml` に `mailpit` サービス追加。image `axllent/mailpit`(タグは実装時に最新安定版を確認)。
- ポート: SMTP `1025`(内部)、Web UI `8025`(ホスト公開)。
- 認証なし、TLS なし(ローカル開発)。永続化不要(再起動で受信トレイは消えてよい)。

### 3.2 Grafana SMTP 設定
- env で設定:
  - `GF_SMTP_ENABLED=true`
  - `GF_SMTP_HOST=mailpit:1025`
  - `GF_SMTP_FROM_ADDRESS=grafana@checkout.local`
  - `GF_SMTP_FROM_NAME=Grafana`
  - `GF_SMTP_SKIP_VERIFY=true`

### 3.3 アラート provisioning(`infra/grafana/provisioning/alerting/`)
Grafana unified alerting の provisioning ファイルを置く。

- **contact point**(`contactpoints.yaml`): `email` タイプ、`addresses: alerts@checkout.local`。Mailpit が宛先に関係なく全メールを受信する。
- **notification policy**(`policies.yaml`): ルートの既定 receiver を上記 email contact point に。
- **alert rules**(`rules.yaml`): フォルダ + 2ルール。データソースは Mimir(uid `mimir`)。
  1. **High error rate**: `sum(rate(http_server_errors_total[1m])) > 0.05`、`for: 1m`。
  2. **High p95 latency**: `histogram_quantile(0.95, sum(rate(http_server_duration_milliseconds_bucket[1m])) by (le)) > 200`、`for: 1m`。
- alerting provisioning を Grafana が読むよう、既存の provisioning マウント(`/etc/grafana/provisioning`)配下に `alerting/` を追加するだけでよい。

> 注: Grafana のアラートルール provisioning は UID・`orgId`・`folder`・`condition`(reduce/threshold を含む `data` 配列)など版依存のスキーマがある。正確な YAML は実装計画時に Grafana 11.2 のドキュメント(Context7)で確定し、起動して発火を検証する。

### 3.4 アプリのレイテンシ注入
- 環境変数 `LATENCY_MS`(既定 0)。`checkout` 処理に `time.Sleep(time.Duration(latencyMs) * time.Millisecond)` を注入する。注入点は `FLAKE_RATE` と同じ `Service` 内(例: `charge` の前後、または `Checkout` 冒頭)。
- 既存パターンに合わせ、`Service` に `latency time.Duration` フィールドを足し、`NewService(flakeRate float64, latency time.Duration)` のように拡張。`main.go` で env をパースして渡す。
- TDD: 「`LATENCY_MS` 相当の遅延を設定すると `Checkout` が概ねその時間以上かかる」ことを、注入したクロック/duration で検証(実時間に依存しすぎないよう、sleep 関数を注入するか、小さな duration で経過時間を測る)。

### 3.5 Logs ダッシュボード(`infra/grafana/provisioning/dashboards/logs.json`)
- データソース: Loki(uid `loki`)。
- テンプレート変数:
  - `service`: `label_values(service_name)`(既定 `checkout-api`)
  - `level`: カスタム(INFO, WARN, ERROR)または `label_values(severity_text)`、複数選択可
  - `trace_id`: テキストボックス(空可)
- Logs パネル(`type: logs`): LogQL 例
  `{service_name="$service"} | json | severity_text=~"$level" | trace_id=~".*$trace_id.*"`
  - 空の `trace_id` でも全件出るようにフィルタを組む(空→`.*`)。
- doc: `06_logs_loki.md` に Explore→Loki の探索手順と Logs ダッシュボードの使い方を追記。

### 3.6 Tempo metrics-generator(サービスグラフ)
- `infra/tempo/tempo.yaml` に `metrics_generator` を追加:
  - `storage.remote_write` → `http://mimir:9009/api/v1/push`(`headers: X-Scope-OrgID: anonymous`、`send_exemplars: true`)
  - `storage.path` / `traces_storage.path`(WAL/一時)
  - `registry.external_labels`(例 `source: tempo`)
- プロセッサ有効化: `overrides.defaults.metrics_generator.processors: [service-graphs, span-metrics]`(Tempo 2.6 のインライン overrides 形式。正確なキーは実装時に確認)。
- 生成される `traces_service_graph_*` / `traces_spanmetrics_*` メトリクスが Mimir に入る。

### 3.7 Tempo データソース拡張(`datasources.yaml`)
- Tempo の jsonData に追加:
  - `serviceMap: { datasourceUid: mimir }`(Service Graph がサービスグラフメトリクスを引く先)
  - `nodeGraph: { enabled: true }`
  - 既存 `tracesToLogsV2` / `tracesToMetrics` は維持。

### 3.8 Traces Drilldown アプリ
- Grafana env に `GF_INSTALL_PLUGINS=grafana-exploretraces-app`。
- 初回起動時にプラグインを DL(要ネット)。オフライン環境では起動失敗ではなくプラグイン欠如として扱い、README に注記。

## 4. ディレクトリ/ファイル変更

```
08_observability/
├── docker-compose.yml                       # + mailpit, grafana(SMTP env, GF_INSTALL_PLUGINS), app(LATENCY_MS)
├── app/
│   └── internal/checkout/
│       ├── checkout.go                       # + latency 注入
│       └── checkout_test.go                  # + latency テスト
│   └── main.go                               # + LATENCY_MS パース
├── infra/
│   ├── tempo/tempo.yaml                      # + metrics_generator / overrides
│   └── grafana/provisioning/
│       ├── datasources/datasources.yaml      # Tempo serviceMap / nodeGraph
│       ├── dashboards/logs.json              # 新規 Logs ダッシュボード
│       └── alerting/
│           ├── contactpoints.yaml            # email → alerts@checkout.local
│           ├── policies.yaml                 # default route → email
│           └── rules.yaml                    # error rate / p95 latency
├── docs/
│   └── 06_logs_loki.md                       # Explore/Logs ダッシュボード追記
│   └── (新規 or 既存に追記) アラート & サービスグラフの解説
└── README.md                                 # ポート(8025)、Mailpit、アラート、Service Graph、Traces Drilldown
```

ドキュメントは新規 doc を増やすか既存に追記するかを実装計画時に決める(章の番号体系を壊さない方針。アラート/サービスグラフは新規 doc 追加が自然なら `10_alerting_and_servicegraph.md` 等として末尾追加し、番号振り直しは避ける)。

## 5. 成功条件(受け入れ基準)

1. `docker compose up -d` で mailpit を含む全コンテナが起動する。
2. Mailpit UI(`http://localhost:8025`)が開く。
3. `FLAKE_RATE=0.8` でアプリを再作成し負荷をかけると、**High error rate** アラートが発火し、Mailpit にメールが届く。
4. `LATENCY_MS=500` でアプリを再作成し負荷をかけると、**High p95 latency** アラートが発火し、Mailpit にメールが届く。
5. Grafana の **Logs ダッシュボード**で service / level / trace_id による絞り込みができ、ログ行から trace へジャンプできる。
6. Grafana Explore → Loki で同等のログ探索ができる(doc どおり)。
7. Explore → Tempo → **Service Graph** に `checkout-frontend → checkout-api` のエッジが表示される(metrics-generator のメトリクスが Mimir にある)。
8. トレース詳細に **Node Graph** タブが出る。
9. **Traces Drilldown** アプリが Grafana に表示される(オンライン環境)。
10. Go テストが通る(`LATENCY_MS` 注入のテストを含む)。

## 6. リスク / 留意点

- **Grafana アラート provisioning のスキーマ**: 版依存で複雑(`data`/`condition`/UID)。実装時に Grafana 11.2 の正確な形式を確認し、起動して発火・メール到達まで検証する。
- **Tempo metrics-generator の overrides 形式**: Tempo 2.6 のインライン overrides キーは版で変わる。実装時に確認し、`traces_service_graph_*` が Mimir に出ることを確認する。
- **サービスグラフのエッジ生成**: frontend(client span)→ backend(server span)のペアリングが必要。ブラウザの fetch span と backend の server span が同一トレースに揃っていること(CORS 修正済みなので前提は満たす)を確認する。
- **プラグイン DL**: `grafana-exploretraces-app` は初回オンライン取得。オフラインでは欠如として扱い README に明記。
- **リソース**: コンテナが9個(+mailpit)になる。メモリ余裕を README に追記。
- **アラート発火の遅延**: `for: 1m` + 評価間隔 + メトリクス反映(~10-15s)で、発火まで1〜2分かかる。検証手順に明記。
