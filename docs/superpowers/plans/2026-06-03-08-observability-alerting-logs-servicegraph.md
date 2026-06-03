# 08_observability 拡張 — Alerting email / Log explorer / Service graph 実装計画

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 既存の `08_observability/` スタックに、(1) Grafana アラート発火→Mailpit へメール、(2) Loki ログ閲覧 UI(Logs ダッシュボード + Explore)、(3) Tempo metrics-generator によるフロント→backend のサービスグラフ + Traces Drilldown アプリ、を追加する。

**Architecture:** Mailpit(開発用 SMTP)を追加し Grafana の SMTP 送信先にする。Grafana の alerting provisioning(contact point / policy / 2 rules)を `/etc/grafana/provisioning/alerting/` に置く。アプリに `LATENCY_MS` 注入を足してレイテンシアラートをデモ可能にする。Tempo の `metrics_generator`(service-graphs / span-metrics)を有効化し Mimir に remote_write、Tempo データソースに serviceMap/nodeGraph を設定。Grafana に Traces Drilldown アプリを install。

**Tech Stack:** Docker Compose, Mailpit, Grafana 11.2.0(unified alerting provisioning), Tempo 2.6.0(metrics-generator), Mimir 2.13.0, Loki 3.2.0, Go 1.26(`log/slog`, `time`).

**作業ディレクトリ:** `08_observability/` 配下(リポジトリルートからの相対)。既存の慣習(compose は host:container ポート、provisioning は `infra/grafana/provisioning/`、Go は go.work + 各モジュール)に従う。

**前提となる既存事実(確認済み):**
- Grafana provisioning マウント: `./infra/grafana/provisioning:/etc/grafana/provisioning:ro`(`alerting/` を足すだけで読まれる)。
- Mimir push エンドポイント: `http://mimir:9009/api/v1/push`(`X-Scope-OrgID: anonymous`、multitenancy 無効)。
- データソース UID: `mimir`(prometheus型/既定), `tempo`, `loki`, `prometheus`。
- 既存メトリクス名: `http_server_errors_total`, `http_server_duration_milliseconds_bucket`(ラベル `http_route`, `http_status_code`, `job`)。
- アプリ: `app/internal/checkout/checkout.go` の `Service`(`NewService(flakeRate float64)`)、`app/main.go` が env を読み `checkout.NewService(...)` を生成。

---

## File Structure

```
08_observability/
├── docker-compose.yml                                  # + mailpit, grafana(SMTP env + GF_INSTALL_PLUGINS), app(LATENCY_MS)
├── app/
│   ├── internal/checkout/checkout.go                   # + WithLatency / sleep 注入
│   ├── internal/checkout/checkout_test.go              # + latency テスト
│   └── main.go                                         # + LATENCY_MS パース
├── infra/
│   ├── tempo/tempo.yaml                                # + metrics_generator + overrides
│   └── grafana/provisioning/
│       ├── datasources/datasources.yaml                # Tempo serviceMap/nodeGraph
│       ├── dashboards/logs.json                        # 新規 Logs ダッシュボード
│       └── alerting/
│           ├── contactpoints.yaml                      # email → mailpit
│           ├── policies.yaml                           # default route → email
│           └── rules.yaml                              # error rate / p95 latency
├── docs/
│   ├── 01_concepts.md                                  # 学習動線に 10 を追加
│   ├── 06_logs_loki.md                                 # Explore + Logs ダッシュボード追記
│   └── 10_alerting_servicegraph.md                     # 新規: アラート + サービスグラフ解説
└── README.md                                           # ポート(8025)、Mailpit、アラート、Service Graph、Drilldown、学習動線
```

---

## Phase 1 — Mailpit + Grafana SMTP / plugins

### Task 1: Add Mailpit, wire Grafana SMTP + plugin

**Files:**
- Modify: `08_observability/docker-compose.yml`

- [ ] **Step 1: Add the `mailpit` service** — insert as a new service (place near grafana). Mailpit listens SMTP on 1025 (in-network) and serves a web inbox on 8025.

```yaml
  mailpit:
    image: axllent/mailpit:v1.20
    ports: ["8025:8025"]
    environment:
      MP_SMTP_AUTH_ALLOW_INSECURE: "true"
```

- [ ] **Step 2: Add SMTP env + plugin install to the `grafana` service.** Replace the grafana `environment:` block with:

```yaml
    environment:
      GF_AUTH_ANONYMOUS_ENABLED: "true"
      GF_AUTH_ANONYMOUS_ORG_ROLE: Admin
      GF_AUTH_DISABLE_LOGIN_FORM: "true"
      GF_SMTP_ENABLED: "true"
      GF_SMTP_HOST: "mailpit:1025"
      GF_SMTP_FROM_ADDRESS: "grafana@checkout.local"
      GF_SMTP_FROM_NAME: "Grafana"
      GF_SMTP_SKIP_VERIFY: "true"
      GF_INSTALL_PLUGINS: "grafana-exploretraces-app"
```

- [ ] **Step 3: Add mailpit to grafana `depends_on`.** Change grafana's `depends_on: [mimir, tempo, loki]` to `depends_on: [mimir, tempo, loki, mailpit]`.

- [ ] **Step 4: Validate compose**

Run: `cd 08_observability && docker compose config >/dev/null && echo COMPOSE_OK`
Expected: `COMPOSE_OK`.

- [ ] **Step 5: Commit**

```bash
git add 08_observability/docker-compose.yml
git commit -m "feat(08_observability): add Mailpit + Grafana SMTP and Traces Drilldown plugin"
```

---

## Phase 2 — App latency injection (TDD)

### Task 2: Add `WithLatency` to checkout service + `LATENCY_MS`

**Files:**
- Modify: `08_observability/app/internal/checkout/checkout.go`
- Modify: `08_observability/app/internal/checkout/checkout_test.go`
- Modify: `08_observability/app/main.go`

Backward-compatible: `NewService(flakeRate float64)` keeps its signature; latency is added via a chained `WithLatency`. A `sleep` field (default `time.Sleep`) makes the delay testable without real waiting.

- [ ] **Step 1: Write the failing test** — append to `checkout_test.go`:

```go
func TestService_Checkout_InjectsLatency(t *testing.T) {
	svc := NewService(0.0).WithLatency(200 * time.Millisecond)
	var slept time.Duration
	svc.sleep = func(d time.Duration) { slept = d } // same-package test can set the unexported field

	if _, err := svc.Checkout(context.Background(), Request{Item: "book", Qty: 1}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if slept != 200*time.Millisecond {
		t.Fatalf("want 200ms injected, got %v", slept)
	}
}

func TestService_Checkout_NoLatencyByDefault(t *testing.T) {
	svc := NewService(0.0)
	called := false
	svc.sleep = func(time.Duration) { called = true }
	if _, err := svc.Checkout(context.Background(), Request{Item: "book", Qty: 1}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Fatal("sleep must not be called when latency is 0")
	}
}
```

Also add `"time"` to the test file's imports.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd 08_observability/app && go test ./internal/checkout/ -run Latency -v`
Expected: FAIL — `svc.WithLatency undefined` / `svc.sleep undefined`.

- [ ] **Step 3: Implement** — edit `checkout.go`:

Add `"time"` to imports. Change the `Service` struct and `NewService`, add `WithLatency`, and sleep at the start of `Checkout`:

```go
type Service struct {
	flakeRate float64
	latency   time.Duration
	rng       func() float64
	sleep     func(time.Duration)
}

func NewService(flakeRate float64) *Service {
	return &Service{flakeRate: flakeRate, rng: rand.Float64, sleep: time.Sleep}
}

// WithLatency makes every Checkout sleep d, so the latency alert can be demoed.
func (s *Service) WithLatency(d time.Duration) *Service {
	s.latency = d
	return s
}
```

In `Checkout`, add the sleep as the first statement after the signature line:

```go
func (s *Service) Checkout(ctx context.Context, r Request) (Result, error) {
	if s.latency > 0 {
		s.sleep(s.latency)
	}
	if err := Validate(r); err != nil {
		return Result{}, err
	}
	if err := s.reserveStock(ctx, r); err != nil {
		return Result{}, err
	}
	if err := s.charge(ctx, r); err != nil {
		return Result{}, err
	}
	return Result{OrderID: fmt.Sprintf("ord-%s-%d", r.Item, r.Qty)}, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd 08_observability/app && go test ./internal/checkout/ -v`
Expected: PASS (existing checkout tests + 2 new latency tests).

- [ ] **Step 5: Wire `LATENCY_MS` in `main.go`** — add `"strconv"` is already imported; add a parse helper and apply `WithLatency`.

Change the service construction line:

```go
	flake := parseFlake(os.Getenv("FLAKE_RATE"))
	svc := checkout.NewService(flake).WithLatency(parseLatency(os.Getenv("LATENCY_MS")))
```

Add this helper next to `parseFlake`:

```go
func parseLatency(s string) time.Duration {
	if s == "" {
		return 0
	}
	ms, err := strconv.Atoi(s)
	if err != nil || ms < 0 {
		return 0
	}
	return time.Duration(ms) * time.Millisecond
}
```

(`time` is already imported in main.go.)

- [ ] **Step 6: Build + full test**

Run: `cd 08_observability/app && go build ./... && go test ./...`
Expected: build OK; all tests PASS.

- [ ] **Step 7: Commit**

```bash
git add 08_observability/app/internal/checkout/checkout.go 08_observability/app/internal/checkout/checkout_test.go 08_observability/app/main.go
git commit -m "feat(08_observability): add LATENCY_MS injection for the latency alert demo"
```

---

## Phase 3 — Alerting provisioning

### Task 3: Contact point, policy, and 2 alert rules

**Files:**
- Create: `08_observability/infra/grafana/provisioning/alerting/contactpoints.yaml`
- Create: `08_observability/infra/grafana/provisioning/alerting/policies.yaml`
- Create: `08_observability/infra/grafana/provisioning/alerting/rules.yaml`

- [ ] **Step 1: Write `contactpoints.yaml`**

```yaml
apiVersion: 1
contactPoints:
  - orgId: 1
    name: email-mailpit
    receivers:
      - uid: email_mailpit
        type: email
        settings:
          addresses: alerts@checkout.local
        disableResolveMessage: false
```

- [ ] **Step 2: Write `policies.yaml`** (root route → the email contact point; short timers so the demo is fast)

```yaml
apiVersion: 1
policies:
  - orgId: 1
    receiver: email-mailpit
    group_by: ['grafana_folder', 'alertname']
    group_wait: 10s
    group_interval: 30s
    repeat_interval: 1h
```

- [ ] **Step 3: Write `rules.yaml`** (two rules; each is an instant Mimir query A fed into a `__expr__` threshold C)

```yaml
apiVersion: 1
groups:
  - orgId: 1
    name: checkout-red
    folder: Checkout Alerts
    interval: 30s
    rules:
      - uid: high_error_rate
        title: High error rate (checkout-api)
        condition: C
        for: 1m
        noDataState: OK
        execErrState: Alerting
        annotations:
          summary: "checkout-api error rate is elevated"
        labels:
          severity: critical
        data:
          - refId: A
            relativeTimeRange: { from: 600, to: 0 }
            datasourceUid: mimir
            model:
              refId: A
              editorMode: code
              expr: "sum(rate(http_server_errors_total[1m]))"
              instant: true
              intervalMs: 1000
              maxDataPoints: 43200
          - refId: C
            relativeTimeRange: { from: 600, to: 0 }
            datasourceUid: __expr__
            model:
              refId: C
              type: threshold
              expression: A
              conditions:
                - evaluator:
                    type: gt
                    params: [0.05]
      - uid: high_p95_latency
        title: High p95 latency (checkout-api)
        condition: C
        for: 1m
        noDataState: OK
        execErrState: Alerting
        annotations:
          summary: "checkout-api p95 latency is high"
        labels:
          severity: warning
        data:
          - refId: A
            relativeTimeRange: { from: 600, to: 0 }
            datasourceUid: mimir
            model:
              refId: A
              editorMode: code
              expr: "histogram_quantile(0.95, sum(rate(http_server_duration_milliseconds_bucket[1m])) by (le))"
              instant: true
              intervalMs: 1000
              maxDataPoints: 43200
          - refId: C
            relativeTimeRange: { from: 600, to: 0 }
            datasourceUid: __expr__
            model:
              refId: C
              type: threshold
              expression: A
              conditions:
                - evaluator:
                    type: gt
                    params: [200]
```

- [ ] **Step 4: Validate YAML**

Run: `cd 08_observability && python3 -c "import yaml,glob; [yaml.safe_load(open(f)) for f in glob.glob('infra/grafana/provisioning/alerting/*.yaml')]; print('YAML OK')"`
Expected: `YAML OK`.

- [ ] **Step 5: Commit**

```bash
git add 08_observability/infra/grafana/provisioning/alerting
git commit -m "feat(08_observability): provision email contact point, policy, and RED alert rules"
```

---

## Phase 4 — Logs dashboard

### Task 4: Logs dashboard with service/level/trace_id filters

**Files:**
- Create: `08_observability/infra/grafana/provisioning/dashboards/logs.json`

The existing dashboards provider (`dashboards.yaml`) already loads every JSON in this folder, so no provider change is needed.

- [ ] **Step 1: Write `logs.json`**

```json
{
  "title": "Logs — checkout-api",
  "schemaVersion": 39,
  "editable": true,
  "time": { "from": "now-15m", "to": "now" },
  "templating": {
    "list": [
      {
        "name": "service",
        "type": "query",
        "datasource": { "type": "loki", "uid": "loki" },
        "query": { "label": "service_name", "stream": "", "type": 1 },
        "refresh": 1,
        "current": { "text": "checkout-api", "value": "checkout-api" }
      },
      {
        "name": "level",
        "type": "custom",
        "multi": true,
        "includeAll": true,
        "allValue": ".+",
        "query": "INFO,WARN,ERROR",
        "current": { "text": "All", "value": "$__all" }
      },
      {
        "name": "trace_id",
        "type": "textbox",
        "current": { "text": "", "value": "" }
      }
    ]
  },
  "panels": [
    {
      "id": 1,
      "title": "Logs",
      "type": "logs",
      "datasource": { "type": "loki", "uid": "loki" },
      "gridPos": { "h": 20, "w": 24, "x": 0, "y": 0 },
      "options": { "showTime": true, "wrapLogMessage": true, "dedupStrategy": "none", "enableLogDetails": true, "sortOrder": "Descending" },
      "targets": [
        {
          "refId": "A",
          "datasource": { "type": "loki", "uid": "loki" },
          "expr": "{service_name=\"$service\"} | json | severity_text=~\"$level\" | trace_id=~\".*$trace_id.*\""
        }
      ]
    }
  ]
}
```

- [ ] **Step 2: Validate JSON**

Run: `cd 08_observability && python3 -m json.tool infra/grafana/provisioning/dashboards/logs.json >/dev/null && echo JSON_OK`
Expected: `JSON_OK`.

- [ ] **Step 3: Commit**

```bash
git add 08_observability/infra/grafana/provisioning/dashboards/logs.json
git commit -m "feat(08_observability): add Logs dashboard (service/level/trace_id filters)"
```

---

## Phase 5 — Tempo service graph

### Task 5: Enable Tempo metrics-generator

**Files:**
- Modify: `08_observability/infra/tempo/tempo.yaml`

- [ ] **Step 1: Append the metrics-generator + overrides blocks** to `tempo.yaml` (keep the existing `server`/`distributor`/`ingester`/`storage` blocks unchanged):

```yaml
metrics_generator:
  registry:
    external_labels:
      source: tempo
  storage:
    path: /var/tempo/generator/wal
    remote_write:
      - url: http://mimir:9009/api/v1/push
        send_exemplars: true
        headers:
          X-Scope-OrgID: anonymous
  traces_storage:
    path: /var/tempo/generator/traces

overrides:
  defaults:
    metrics_generator:
      processors: [service-graphs, span-metrics]
```

- [ ] **Step 2: Validate YAML**

Run: `cd 08_observability && python3 -c "import yaml; yaml.safe_load(open('infra/tempo/tempo.yaml')); print('YAML OK')"`
Expected: `YAML OK`.

- [ ] **Step 3: Commit**

```bash
git add 08_observability/infra/tempo/tempo.yaml
git commit -m "feat(08_observability): enable Tempo metrics-generator (service-graphs + span-metrics) to Mimir"
```

---

### Task 6: Tempo datasource — service map + node graph

**Files:**
- Modify: `08_observability/infra/grafana/provisioning/datasources/datasources.yaml`

- [ ] **Step 1: Extend the Tempo datasource `jsonData`.** Replace the Tempo entry's `jsonData:` block (currently `tracesToLogsV2` + `tracesToMetrics`) with one that adds `serviceMap` and `nodeGraph`:

```yaml
  - name: Tempo
    type: tempo
    uid: tempo
    access: proxy
    url: http://tempo:3200
    jsonData:
      tracesToLogsV2:
        datasourceUid: loki
        filterByTraceID: true
      tracesToMetrics:
        datasourceUid: mimir
      serviceMap:
        datasourceUid: mimir
      nodeGraph:
        enabled: true
```

- [ ] **Step 2: Validate YAML**

Run: `cd 08_observability && python3 -c "import yaml; yaml.safe_load(open('infra/grafana/provisioning/datasources/datasources.yaml')); print('YAML OK')"`
Expected: `YAML OK`.

- [ ] **Step 3: Commit**

```bash
git add 08_observability/infra/grafana/provisioning/datasources/datasources.yaml
git commit -m "feat(08_observability): Tempo datasource serviceMap + nodeGraph for service graph"
```

---

## Phase 6 — Documentation

### Task 7: New doc `10_alerting_servicegraph.md`

**Files:**
- Create: `08_observability/docs/10_alerting_servicegraph.md`

- [ ] **Step 1: Write the doc** (日本語・である調・末尾に「まとめ / 関連 doc」)。必須内容:
  - **アラート**: Grafana unified alerting の構成(contact point / notification policy / alert rule)。本章の `infra/grafana/provisioning/alerting/` の3ファイルを引用解説。rule の `data`(refId A=Mimir instant query, refId C=`__expr__` threshold, `condition: C`)の意味。`for: 1m` と評価間隔・メトリクス反映で発火まで1〜2分かかること。
  - **メール**: Mailpit(SMTP :1025 / UI :8025)と Grafana の `GF_SMTP_*`。宛先固定 + Mailpit 全受信の仕組み。
  - **デモ手順**: `FLAKE_RATE=0.8` でエラー率アラート、`LATENCY_MS=500` でレイテンシアラート → `http://localhost:8025` で受信確認。
  - **サービスグラフ**: Tempo metrics-generator(service-graphs/span-metrics)→ Mimir、Tempo datasource の `serviceMap`/`nodeGraph`。Explore→Tempo→Service Graph で `checkout-frontend → checkout-api`、トレース詳細の Node Graph タブ、Traces Drilldown アプリ。生成メトリクス `traces_service_graph_*` / `traces_spanmetrics_*` に言及。
  - 末尾「まとめ / 関連 doc」: [05_metrics_prom_mimir.md], [06_logs_loki.md], [07_grafana_correlation.md], [08_collector.md] へのリンク。

- [ ] **Step 2: Commit**

```bash
git add 08_observability/docs/10_alerting_servicegraph.md
git commit -m "docs(08_observability): 10_alerting_servicegraph"
```

---

### Task 8: Update logs doc + learning paths

**Files:**
- Modify: `08_observability/docs/06_logs_loki.md`
- Modify: `08_observability/docs/01_concepts.md`
- Modify: `08_observability/README.md`

- [ ] **Step 1: Append to `06_logs_loki.md`** a section「## Grafana でログを探索する」covering: (a) Explore → データソース Loki → クエリ `{service_name="checkout-api"} | json`、`| trace_id="..."` で絞り込み、ログ行から Tempo へジャンプ。(b) Logs ダッシュボード(`infra/grafana/provisioning/dashboards/logs.json`)のテンプレート変数 service/level/trace_id の使い方。Place it before the existing「まとめ / 関連 doc」section.

- [ ] **Step 2: Add doc 10 to `01_concepts.md` の学習動線テーブル.** After the `09_oss_landscape.md` row, add:

```
| **10_alerting_servicegraph.md** | Grafana アラートメール(Mailpit)・サービスグラフ(Tempo metrics-generator) |
```

- [ ] **Step 3: Add doc 10 to the README learning path.** After the `9. [09_oss_landscape.md]...` line add:

```
10. [10_alerting_servicegraph.md](docs/10_alerting_servicegraph.md) — Grafana アラートメール(Mailpit)・サービスグラフ・Traces Drilldown
```

- [ ] **Step 4: Commit**

```bash
git add 08_observability/docs/06_logs_loki.md 08_observability/docs/01_concepts.md 08_observability/README.md
git commit -m "docs(08_observability): log exploration section + learning path for doc 10"
```

---

### Task 9: README — ports, mailpit, dashboards, known limitations

**Files:**
- Modify: `08_observability/README.md`

- [ ] **Step 1: Add Mailpit to the access table** (`## アクセス先一覧`): add a row

```
| Mailpit | http://localhost:8025 | アラートメールの受信トレイ(開発用 SMTP) |
```

- [ ] **Step 2: Add the Logs dashboard to the dashboards table** (`## Grafana のダッシュボードとデータソース`): add a row

```
| Logs — checkout-api | ログを service/level/trace_id で絞り込み |
```

- [ ] **Step 3: Append to `## 既知の制約`** these bullets:

```
- **アラート発火の遅延**: ルールは `for: 1m` + 評価間隔 30s + メトリクス反映(~10-15s)のため、`FLAKE_RATE`/`LATENCY_MS` を上げてから発火・メール到達まで概ね1〜2分かかる。
- **Traces Drilldown プラグイン**: `grafana-exploretraces-app` は Grafana 初回起動時にオンライン取得する。オフライン環境ではアプリが表示されない。
- **サービスグラフの初期表示遅延**: Tempo metrics-generator が `traces_service_graph_*` を Mimir に書き、Grafana がそれを引くまで、トラフィック発生から数十秒のラグがある。
```

- [ ] **Step 4: Add quickstart hints** — in the quickstart code block (near the `FLAKE_RATE=0.8` line), add:

```
# レイテンシアラートのデモ (p95 を押し上げる)
LATENCY_MS=500 docker compose up -d --no-deps app
# 受信したアラートメールを確認
open http://localhost:8025
```

- [ ] **Step 5: Commit**

```bash
git add 08_observability/README.md
git commit -m "docs(08_observability): README updates for Mailpit, Logs dashboard, alert limitations"
```

---

## Phase 7 — End-to-end verification

### Task 10: Bring up the stack and verify all acceptance criteria

**Files:** none (verification; fix earlier configs if needed)

- [ ] **Step 1: Bring up (with plugin download)**

Run: `cd 08_observability && docker compose up -d --build`
Expected: all containers (incl. mailpit, grafana with plugin) start. If grafana fails to install the plugin (offline), note it and continue.

- [ ] **Step 2: Mailpit UI reachable (criterion 2)**

Run: `sleep 30 && curl -s -o /dev/null -w "mailpit=%{http_code}\n" http://localhost:8025`
Expected: `mailpit=200`.

- [ ] **Step 3: Alert rules + contact point loaded (criteria 3/4 prep)**

Run: `curl -s http://localhost:3001/api/v1/provisioning/alert-rules | python3 -c "import sys,json;print([r['title'] for r in json.load(sys.stdin)])"`
Expected: list contains `High error rate (checkout-api)` and `High p95 latency (checkout-api)`.

- [ ] **Step 4: Fire the error-rate alert and confirm email (criterion 3)**

Run:
```bash
cd 08_observability
FLAKE_RATE=0.8 docker compose up -d --no-deps app
sleep 20 && make load && sleep 90
curl -s 'http://localhost:8025/api/v1/messages' | python3 -c "import sys,json;d=json.load(sys.stdin);print('messages:', d.get('total'))"
```
Expected: `messages:` > 0 (Grafana sent an alert email to Mailpit). If 0, check `http://localhost:3001` → Alerting → rules state is `Firing`, and that `http_server_errors_total` exists in Mimir.

- [ ] **Step 5: Fire the latency alert (criterion 4)**

Run:
```bash
cd 08_observability
LATENCY_MS=500 docker compose up -d --no-deps app
sleep 20 && make load && sleep 90
curl -s 'http://localhost:8025/api/v1/messages' | python3 -c "import sys,json;print('messages:', json.load(sys.stdin).get('total'))"
```
Expected: message count increased (latency alert fired). Reset afterwards: `docker compose up -d --no-deps app`.

- [ ] **Step 6: Logs dashboard + Explore (criteria 5/6)**

Open `http://localhost:3001` → Dashboards → "Logs — checkout-api". Confirm logs appear, and changing `level`/`trace_id` filters the panel. Then Explore → Loki → `{service_name="checkout-api"} | json` returns lines.
Expected: filtered logs render; trace_id links jump to Tempo.

- [ ] **Step 7: Service graph edge (criteria 7/8)**

First generate cross-origin traffic from the browser (`http://localhost:5174`, click Checkout a few times) so a `checkout-frontend → checkout-api` trace exists. Then verify the generator metric reached Mimir:

Run: `curl -s -H 'X-Scope-OrgID: anonymous' 'http://localhost:9009/prometheus/api/v1/query?query=traces_service_graph_request_total' | python3 -c "import sys,json;print('series:', len(json.load(sys.stdin)['data']['result']))"`
Expected: `series:` > 0. Then in Grafana Explore → Tempo → "Service Graph" tab shows nodes/edges; open a trace → "Node Graph" tab is present.

- [ ] **Step 8: Traces Drilldown app (criterion 9, online only)**

Run: `curl -s http://localhost:3001/api/plugins/grafana-exploretraces-app/settings | python3 -c "import sys,json;d=json.load(sys.stdin);print('enabled:', d.get('enabled'))" 2>/dev/null || echo "plugin not installed (offline?)"`
Expected: `enabled: True` (or note offline).

- [ ] **Step 9: Go tests still pass (criterion 10)**

Run: `cd 08_observability/app && go test ./...`
Expected: all PASS.

- [ ] **Step 10: Tear down**

Run: `cd 08_observability && docker compose down -v`

- [ ] **Step 11: Commit any fixes discovered during verification**

```bash
git add 08_observability
git commit -m "fix(08_observability): align alerting/service-graph configs after end-to-end verification"
```

---

## Self-Review Notes (completed by plan author)

- **Spec coverage:** Mailpit+SMTP(Task1) / LATENCY_MS(Task2) / contact point+policy+2 rules(Task3) / Logs dashboard(Task4) / Tempo metrics-generator(Task5) / Tempo serviceMap+nodeGraph(Task6) / Traces Drilldown plugin(Task1 env) / docs(Task7-9) / e2e 受け入れ基準1-10(Task10)。spec 全節に対応タスクあり。
- **Placeholder scan:** すべての設定/コードは実値。版依存箇所(alert rule `data` モデル, Tempo overrides)は Grafana v11.2.2 / Tempo の実ドキュメント形式に基づき具体化済み。残る不確実性(プラグインのオフライン取得、発火の正確な閾値の当たり)は Task10 の検証手順で吸収。
- **Type consistency:** `NewService(flakeRate float64)` は不変、`WithLatency(d time.Duration) *Service` を追加(既存テスト・呼び出しは無改変)。`sleep`/`latency` フィールドは同一パッケージテストからアクセス。`parseLatency` は main.go に追加し `time` 既存 import を利用。
- **既存への影響:** Task6 は Tempo datasource の jsonData を「置換」する形だが tracesToLogsV2/tracesToMetrics を保持。dashboards provider は既存のまま logs.json を自動ロード。alerting は新規ディレクトリ追加のみ。
```
