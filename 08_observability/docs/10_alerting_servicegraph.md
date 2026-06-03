# 10_alerting_servicegraph: Grafana アラートとサービスグラフ

本 doc では2つのテーマを扱う。

1. **Grafana unified alerting** — アラートルールをコードで管理し、Mailpit 経由でメール通知を受け取る仕組み
2. **Tempo metrics-generator + サービスグラフ** — トレースからメトリクスを自動生成してサービス間の依存関係をグラフで可視化する仕組み

---

## Grafana unified alerting の全体像

Grafana のアラートは3つの概念で構成される。

```
Alert Rule (ルール)
  │  条件を満たしたら Firing 状態に遷移
  ▼
Notification Policy (通知ポリシー)
  │  どのルールをどの宛先に送るかをルーティング
  ▼
Contact Point (コンタクトポイント)
  │  実際の通知先(Email / Slack / PagerDuty 等)
  └► メール送信 → SMTP サーバー → 受信トレイ
```

| 概念 | 役割 |
|---|---|
| **Alert Rule** | PromQL / LogQL 等で評価条件を定義。`for` でその状態が何秒続いたら Firing にするかを指定 |
| **Notification Policy** | ラベルマッチングで Firing になったアラートをどの Contact Point に送るかを決定 |
| **Contact Point** | Email / Slack 等の通知先の設定。SMTP アドレスや Webhook URL を保持 |

---

## アラート provisioning の3ファイル

`infra/grafana/provisioning/alerting/` に3つのファイルを配置し、Grafana 起動時にアラートを自動登録している。

### contactpoints.yaml — 通知先の定義

```yaml
apiVersion: 1
contactPoints:
  - orgId: 1
    name: email-mailpit
    receivers:
      - uid: email-mailpit
        type: email
        settings:
          addresses: alerts@checkout.local
```

`alerts@checkout.local` が固定の宛先だ。Mailpit が全メールを受信するため、実際の宛先アドレスが存在しなくても届く。

### policies.yaml — ルーティングポリシー

```yaml
apiVersion: 1
policies:
  - orgId: 1
    receiver: email-mailpit
    group_wait: 10s
    group_interval: 30s
    repeat_interval: 1h
```

| キー | 値 | 意味 |
|---|---|---|
| `receiver` | `email-mailpit` | すべてのアラートをこの Contact Point に送る |
| `group_wait` | 10s | 同グループ内の最初の通知を10秒待ってまとめて送る |
| `group_interval` | 30s | 同グループの通知を30秒ごとにまとめる |
| `repeat_interval` | 1h | 解決されない場合1時間ごとに再通知 |

### rules.yaml — アラートルールの定義

フォルダ名 `"Checkout Alerts"`、評価間隔 30s で2つのルールを定義している。

**ルールの `data` フィールドの読み方**

各ルールは `data` リストに2つのクエリ参照を持つ。

| refId | datasourceUid | 役割 |
|---|---|---|
| `A` | `mimir` | Mimir に対する PromQL instant query。メトリクスを取得する |
| `C` | `__expr__` | Grafana 内部の式エンジン。refId A の結果に閾値判定を適用する |

`condition: C` は「refId C の結果が条件を満たしたときにアラートを Firing にする」という意味だ。`__expr__` はバックエンドへのクエリではなく Grafana が内部で評価する式を表す特殊な datasourceUid だ。

`noDataState: OK` はデータが存在しない場合(カウンタ系列未生成など)をアラートなしとして扱う設定だ。エラーメトリクスが初回の観測まで存在しない挙動([05_metrics_prom_mimir.md](./05_metrics_prom_mimir.md) 参照)に対応している。

**ルール1: High error rate**

```yaml
title: High error rate
condition: C
for: 1m
noDataState: OK
data:
  - refId: A
    datasourceUid: mimir
    model:
      expr: sum(rate(http_server_errors_total[1m])) > 0.05
      instant: true
  - refId: C
    datasourceUid: __expr__
    model:
      type: threshold
      expression: A
      conditions:
        - evaluator:
            type: gt
            params: [0]
```

`http_server_errors_total` の1分間の rate の合計が 0.05 rps を超えた状態が1分間継続すると Firing になる。

**ルール2: High p95 latency**

```yaml
title: High p95 latency
condition: C
for: 1m
noDataState: OK
data:
  - refId: A
    datasourceUid: mimir
    model:
      expr: >
        histogram_quantile(0.95,
          sum(rate(http_server_duration_milliseconds_bucket[1m])) by (le))
        > 200
      instant: true
  - refId: C
    datasourceUid: __expr__
    model:
      type: threshold
      expression: A
      conditions:
        - evaluator:
            type: gt
            params: [0]
```

p95 レイテンシが 200ms を超えた状態が1分間継続すると Firing になる。

---

## メール通知: Mailpit と GF_SMTP_*

### Mailpit とは

**Mailpit**(`axllent/mailpit`)は開発環境用の SMTP サーバーで、送信されたメールをすべてキャッチして Web UI で確認できるツールだ。本番環境の SMTP サーバーを使わずにアラートメールの動作確認ができる。

```
Grafana
  │  SMTP(mailpit:1025)
  ▼
[Mailpit コンテナ]
  │  全メールを内部でキャプチャ
  ▼
http://localhost:8025  ← ブラウザで受信メールを確認
```

### Grafana の SMTP 設定

Grafana コンテナには以下の環境変数で SMTP を設定している。

| 環境変数 | 値 | 意味 |
|---|---|---|
| `GF_SMTP_ENABLED` | `true` | SMTP 送信を有効化 |
| `GF_SMTP_HOST` | `mailpit:1025` | コンテナ内部 SMTP エンドポイント |
| `GF_SMTP_FROM_ADDRESS` | `grafana@checkout.local` | 送信元アドレス |
| `GF_SMTP_SKIP_VERIFY` | `true` | TLS 証明書検証をスキップ(開発用) |

`alerts@checkout.local` は実在しないドメインのアドレスだが、Mailpit がすべてのメールを受信するため問題ない。

### 受信確認の手順

```bash
# Mailpit の受信トレイをブラウザで開く
open http://localhost:8025
```

アラートが Firing になると `grafana@checkout.local` から `alerts@checkout.local` 宛のメールが届く。件名にはアラートルール名と状態(Firing / Resolved)が含まれる。

---

## デモ手順とアラート発火までのタイムライン

### エラー率アラートのデモ

```bash
# エラー率を 80% に設定してアプリだけ再起動
FLAKE_RATE=0.8 docker compose up -d --no-deps app

# 負荷をかけてメトリクスを溜める
make load

# Mailpit でメール受信を確認
open http://localhost:8025
```

### レイテンシアラートのデモ

```bash
# 処理に 500ms の sleep を注入してアプリだけ再起動
LATENCY_MS=500 docker compose up -d --no-deps app

# 負荷をかける
make load

# Mailpit で受信確認
open http://localhost:8025
```

`LATENCY_MS` 環境変数は checkout 処理に `time.Sleep` を挿入するデモ用ノブだ。`FLAKE_RATE` と同じく、`docker compose up -d --no-deps app` でアプリだけ再起動すれば他のスタックコンポーネントに影響しない。

### 発火までのタイムライン

```
ノブ設定 → アプリ再起動
  └─ ~10〜15秒 → OTel SDK export → Prometheus scrape → Mimir 反映
       └─ 30秒 → Grafana がアラートルールを評価(評価間隔 30s)
            └─ 1分 → for:1m の継続条件を満たす → Firing
                 └─ ~10秒 → Notification Policy の group_wait → メール送信
                      └─ Mailpit に到達 (合計: 概ね1〜2分)
```

`for:1m` は「条件が少なくとも1分間継続したとき」という意味で、一時的なスパイクでの誤検知を防ぐ。ノブを上げてからメールが届くまで1〜2分かかるのはこのためだ。

---

## サービスグラフ: Tempo metrics-generator

### Tempo metrics-generator とは

Tempo 2.x に組み込まれた **metrics-generator** は、取り込んだトレースを解析してメトリクスを自動生成し、外部の Prometheus 互換ストレージに書き出す機能だ。トレースを1件1件手動で解析しなくても、サービス間の呼び出しパターンやレイテンシ分布をメトリクスとして継続的に得られる。

### tempo.yaml の設定

```yaml
metrics_generator:
  registry:
    external_labels:
      source: tempo
  storage:
    remote_write:
      - url: http://mimir:9009/api/v1/push
        headers:
          X-Scope-OrgID: anonymous
        send_exemplars: true

overrides:
  defaults:
    metrics_generator:
      processors: [service-graphs, span-metrics]
```

| プロセッサ | 生成するメトリクス | 内容 |
|---|---|---|
| `service-graphs` | `traces_service_graph_*` | サービス間のコール数・レイテンシ・エラー率。サービスグラフノードの描画に使う |
| `span-metrics` | `traces_spanmetrics_*` | span 単位の duration / count。操作ごとの RED メトリクスとして使える |

生成されたメトリクスは `remote_write` で Mimir に書き込まれる。`X-Scope-OrgID: anonymous` はマルチテナント認証のヘッダで、本章の Mimir 設定に合わせた値だ。

### Tempo datasource の serviceMap と nodeGraph

Grafana の Tempo データソースに以下の設定が入っている。

```json
{
  "serviceMap": {
    "datasourceUid": "mimir"
  },
  "nodeGraph": {
    "enabled": true
  }
}
```

`serviceMap.datasourceUid: mimir` を指定することで、Tempo のサービスグラフ表示が Mimir から `traces_service_graph_*` メトリクスを参照するようになる。`nodeGraph.enabled: true` はトレース詳細画面に「Node Graph」タブを追加する設定だ。

### Grafana Explore でのサービスグラフ確認

**サービスグラフ(Explore → Tempo)**

1. Grafana Explore を開く (`http://localhost:3001/explore`)
2. データソースに **Tempo** を選択
3. 「**Service Graph**」タブをクリック
4. `checkout-frontend → checkout-api` のノードグラフが表示される

ノード間のエッジにはリクエスト数・エラー率・レイテンシが表示される。エッジをクリックするとそのサービス間の関連トレースに直接ジャンプできる。

**Node Graph(トレース詳細)**

個別のトレースを開いた画面で「**Node Graph**」タブをクリックすると、そのトレース内の span 間の依存関係がノードグラフで表示される。`nodeGraph.enabled: true` が有効になっているときのみ表示される。

---

## Traces Drilldown アプリ

Grafana に **Traces Drilldown**(`grafana-exploretraces-app`)プラグインをインストールすることで、トレースの探索体験が強化される。

```yaml
# grafana コンテナの環境変数
GF_INSTALL_PLUGINS: grafana-exploretraces-app
```

このプラグインは Grafana 起動時にオンラインで取得される。**オフライン環境では初回取得ができないため、Traces Drilldown のメニューが表示されない**ことがある。

インストール済みの場合、Grafana の左メニューに「Drilldown → Traces」が追加される。Explore の素のビューより視覚的なトレース分析ができる。

---

## 生成メトリクスと確認クエリ

Mimir に書き込まれた metrics-generator のメトリクスは Grafana Explore から直接クエリできる。

```promql
# サービスグラフ: checkout-api へのリクエスト数 (rate)
rate(traces_service_graph_request_total[1m])

# サービスグラフ: サービス間のエラー率
rate(traces_service_graph_request_failed_total[1m])
  / rate(traces_service_graph_request_total[1m])

# span メトリクス: 操作ごとの p95 レイテンシ
histogram_quantile(0.95,
  sum(rate(traces_spanmetrics_duration_milliseconds_bucket[1m])) by (le, operation))
```

```bash
# Mimir への直接クエリ例
curl -s -H 'X-Scope-OrgID: anonymous' \
  'http://localhost:9009/prometheus/api/v1/label/__name__/values' \
  | jq '.data[] | select(startswith("traces_"))'
```

---

## まとめ / 関連 doc

- Grafana unified alerting は「Contact Point → Notification Policy → Alert Rule」の3層で構成される。provisioning ファイルでコードとして管理できる。
- ルールの `data` は refId A(Mimir instant query)と refId C(`__expr__` 閾値判定)の組み合わせ。`condition: C`、`for: 1m` で継続評価を行う。
- Mailpit は開発用 SMTP キャッチサーバー。`GF_SMTP_HOST=mailpit:1025` で Grafana の送信先に設定し、`http://localhost:8025` で受信確認できる。
- ノブを上げてからアラートメールが届くまで1〜2分のタイムラグがある(`for:1m` + 評価間隔30s + メトリクス反映)。
- Tempo metrics-generator の `service-graphs` / `span-metrics` プロセッサがトレースから `traces_service_graph_*` / `traces_spanmetrics_*` メトリクスを生成し Mimir に書き込む。
- Tempo datasource の `serviceMap.datasourceUid: mimir` + `nodeGraph.enabled: true` で Explore → Tempo → Service Graph タブが有効になる。

**関連 doc:**
- [05_metrics_prom_mimir.md](./05_metrics_prom_mimir.md) — RED メトリクス・PromQL・Mimir の役割
- [06_logs_loki.md](./06_logs_loki.md) — 構造化ログ・trace_id 相関・LogQL
- [07_grafana_correlation.md](./07_grafana_correlation.md) — データソース相関設定・トレース↔ログ・メトリクスの往復
- [08_collector.md](./08_collector.md) — Collector のパイプライン設定・メトリクス経路
