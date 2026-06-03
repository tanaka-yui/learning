# 06_grafana_correlation: Grafana 相関設定とダッシュボード

## 3種のデータソース

本章の Grafana は **Mimir / Tempo / Loki** を provisioning で自動設定する。コンテナ起動時に `infra/grafana/provisioning/datasources/datasources.yaml` が読み込まれ、ログイン不要の anonymous Admin で即座に使える状態になる。

| データソース | UID | type | URL | 役割 |
|---|---|---|---|---|
| Mimir | `mimir` | prometheus | `http://mimir:9009/prometheus` | メトリクス長期保存 |
| Tempo | `tempo` | tempo | `http://tempo:3200` | トレース |
| Loki | `loki` | loki | `http://loki:3100` | ログ |

Mimir は Prometheus 互換 API を持つため type は `prometheus` となる。デフォルトデータソースは Mimir に設定してある。

---

## 相関設定の詳細

3シグナルのジャンプを支える設定が provisioning YAML に記述されている。

### Mimir: Exemplar → Tempo

```yaml
# datasources.yaml (Mimir 抜粋)
- name: Mimir
  type: prometheus
  uid: mimir
  isDefault: true
  url: http://mimir:9009/prometheus
  jsonData:
    httpHeaderName1: X-Scope-OrgID
    exemplarTraceIdDestinations:
      - name: trace_id
        datasourceUid: tempo
  secureJsonData:
    httpHeaderValue1: anonymous
```

`exemplarTraceIdDestinations` は、メトリクスグラフ上の Exemplar ドット(◆)をクリックしたときに `trace_id` フィールドを使って Tempo に飛ぶ設定だ。`httpHeaderName1` / `httpHeaderValue1` は Mimir の `X-Scope-OrgID: anonymous` ヘッダを毎リクエストに付与する。

### Tempo: Traces → Logs / Traces → Metrics

```yaml
# datasources.yaml (Tempo 抜粋)
- name: Tempo
  type: tempo
  uid: tempo
  url: http://tempo:3200
  jsonData:
    tracesToLogsV2:
      datasourceUid: loki
      filterByTraceID: true
    tracesToMetrics:
      datasourceUid: mimir
```

`tracesToLogsV2.filterByTraceID: true` により、Tempo でトレースを開いたときに「Logs for this span」リンクが表示され、同じ `trace_id` を持つ Loki のログ行に直接ジャンプできる。`tracesToMetrics` は span からメトリクスダッシュボードへのリンクを生成する。

### Loki: Logs → Traces

```yaml
# datasources.yaml (Loki 抜粋)
- name: Loki
  type: loki
  uid: loki
  url: http://loki:3100
  jsonData:
    derivedFields:
      - name: trace_id
        matcherRegex: '"trace_id":"([a-f0-9]{32})"'
        url: '$${__value.raw}'
        datasourceUid: tempo
```

`derivedFields` はログ行のテキストを正規表現でスキャンし、`trace_id` フィールドが見つかった場合に Tempo へのリンクアイコンを行末に表示する。`datasourceUid: tempo` で飛び先を Tempo に固定する。

---

## Explore での3本柱往復ワークフロー

Grafana の「Explore」モードが相関調査の中心だ。

### Metric → Trace → Log

```
1. Explore → データソース: Mimir
   PromQL: histogram_quantile(0.95, sum(rate(http_server_duration_milliseconds_bucket[5m])) by (le))
   → グラフにレイテンシのスパイクを発見

2. スパイク付近の Exemplar ドット(◆)をクリック
   → Tempo が開き、該当リクエストのスパンツリーを表示

3. スパンツリーで遅い span を選択 → 「Logs for this span」をクリック
   → Loki が開き、同じ trace_id を持つログ行を表示

4. ログ行で例外スタックトレースやリクエストパラメータを確認
```

### Log → Trace

```
1. Explore → データソース: Loki
   LogQL: {service_name="checkout-api"} | json | level = "ERROR"
   → エラーログ行を発見

2. ログ行末の Tempo リンクアイコンをクリック (derivedFields)
   → 同じ trace_id の Tempo トレースにジャンプ
```

---

## RED ダッシュボード

`infra/grafana/provisioning/dashboards/red.json` が provisioning で自動配布される。Grafana 起動時に「RED Dashboard」として利用可能だ。

### 3パネルの読み方

| パネル | PromQL | 読み方 |
|---|---|---|
| Request Rate | `sum(rate(http_server_requests_total[1m]))` | 1分平均のリクエスト/秒。スパイクやドロップで異常を察知 |
| Error Rate | `sum(rate(http_server_errors_total[1m]))` | エラー増加で SLO 違反のリスクを検知 |
| p95 Latency | `histogram_quantile(0.95, ...)` | 95 パーセンタイルのレイテンシ。ユーザ体験の劣化指標 |

`FLAKE_RATE=0.8 make up` でエラー率を意図的に上げると Error Rate パネルの値が増加し、同時に Tempo にエラー span・Loki にエラーログが入る様子を3本柱で横断確認できる。

---

## provisioning でダッシュボードを配布する方法

Grafana は起動時に以下のディレクトリを読み込む。

```
infra/grafana/provisioning/
├── datasources/
│   └── datasources.yaml    # データソース定義
└── dashboards/
    ├── dashboards.yaml     # ダッシュボードのプロバイダ設定
    └── red.json            # ダッシュボード本体 (JSON)
```

`dashboards.yaml` でフォルダとパスを指定し、JSON ファイルを置くだけでダッシュボードが自動インポートされる。

```yaml
# infra/grafana/provisioning/dashboards/dashboards.yaml
apiVersion: 1
providers:
  - name: default
    folder: ''
    type: file
    options:
      path: /etc/grafana/provisioning/dashboards
```

JSON はダッシュボード画面の「Share → Export → Save to file」で取得できる。チームへの配布は Git にコミットするだけでよく、Grafana UI での手動インポート操作が不要になる。

---

## Grafana の anonymous アクセス設定

本章の Grafana はログインフォームを無効化し、anonymous Admin で動作する。

```ini
# grafana.ini (抜粋)
[auth.anonymous]
enabled = true
org_role = Admin

[auth]
disable_login_form = true
```

学習・デモ用途の設定であり、本番環境では必ず認証を有効にすること。

---

## まとめ / 関連 doc

- 3データソース(Mimir / Tempo / Loki)の provisioning により、起動直後から相関ジャンプが使える。
- `exemplarTraceIdDestinations` (Mimir→Tempo)、`tracesToLogsV2` (Tempo→Loki)、`derivedFields` (Loki→Tempo) の3設定が双方向ナビゲーションを実現する。
- RED ダッシュボードは provisioning で Git 管理でき、チームへの配布が容易だ。

**関連 doc:**
- [01_concepts.md](./01_concepts.md) — 3本柱の概念と調査フロー
- [04_metrics_prom_mimir.md](./04_metrics_prom_mimir.md) — Mimir とメトリクス経路の詳細
- [05_logs_loki.md](./05_logs_loki.md) — Loki のラベル設計と LogQL
- [03_traces_e2e.md](./03_traces_e2e.md) — Tempo でのトレース確認方法
