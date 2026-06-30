# Linkerd — Rust micro-proxy・SMI ServiceProfile・TrafficSplit

## Linkerd の設計思想

Linkerd は **シンプル・軽量・運用コスト最小** を設計原則とするサービスメッシュである。
Istio が豊富な機能を Envoy に委ねるのに対し、Linkerd は Rust 製の **linkerd2-proxy** という超軽量プロキシを採用する。

| 観点 | Linkerd | Istio (sidecar) |
|---|---|---|
| proxy | linkerd2-proxy (Rust) | Envoy (C++) |
| sidecar CPU (idle) | ~2 m | ~10–15 m |
| sidecar メモリ (idle) | ~20 MB | ~50–60 MB |
| 設定の複雑さ | シンプル（CLI 中心） | CRD が多岐にわたる |
| L7 traffic management | ServiceProfile / TrafficSplit | VirtualService / DestinationRule |
| 可観測性 | linkerd viz (Prometheus + Grafana) | Kiali + Jaeger + Grafana |

## linkerd2-proxy

linkerd2-proxy は Linkerd プロジェクトがゼロから書いた非同期 Rust proxy で、Tokio + Tower スタックを使う。
Envoy と異なり Linkerd 専用に最適化されており、xDS のような汎用 config 機構は持たない代わりに footprint が非常に小さい。

## ServiceProfile — retries と timeout

`ServiceProfile` は Linkerd 固有の CRD でルートごとの振る舞いを定義する。

```yaml
# manifests/overlays/linkerd/serviceprofile.yaml
apiVersion: linkerd.io/v1alpha2
kind: ServiceProfile
metadata:
  name: api.linkerd-app.svc.cluster.local
  namespace: linkerd-app
spec:
  retryBudget:
    retryRatio: 0.2        # リトライ率 20% 上限
    minRetriesPerSecond: 10
    ttl: 10s
  routes:
  - name: echo
    condition:
      method: GET
      pathRegex: "/api/v[12]/echo"
    timeout: 2s
    isRetryable: true
```

`retryBudget` はリトライによる負荷増大を防ぐレート制限。`isRetryable: true` でリトライ可能と明示する。
`timeout` は individual request に適用される。

## TrafficSplit — カナリア

**SMI (Service Mesh Interface)** の `TrafficSplit` CRD でトラフィック分割を定義する。

```yaml
# manifests/overlays/linkerd/trafficsplit.yaml
apiVersion: split.smi-spec.io/v1alpha2
kind: TrafficSplit
metadata:
  name: api
  namespace: linkerd-app
spec:
  service: api        # クライアントが宛先にする Service
  backends:
  - service: api-v1
    weight: 90
  - service: api-v2
    weight: 10
```

> **重要 (Linkerd edge 版)**: SMI `TrafficSplit` CRD は Linkerd の最新 edge リリース (2.15+) では同梱されなくなった。
> `kubectl apply -f https://raw.githubusercontent.com/servicemeshinterface/smi-spec/main/apis/v1alpha2/trafficsplit.yaml`
> で CRD を別途インストールする必要がある。Linkerd stable (2.14 系) では引き続き同梱されている。

## linkerd viz — 観測

```bash
# viz 拡張のインストール
linkerd viz install | kubectl apply -f -

# ダッシュボード起動
linkerd viz dashboard &

# 特定の Deployment のライブメトリクス表示
linkerd viz stat deploy -n linkerd-app

# リクエストのタップ（リアルタイム確認）
linkerd viz tap deploy/api -n linkerd-app
```

`linkerd viz stat` は SUCCESS_RATE・RPS・LATENCY_P50/P95/P99 を表示する。Grafana ダッシュボードも自動で追加される。

## 別クラスタで動かす理由

Linkerd は `learning-linkerd` クラスタで動かしている（`kind/cluster-linkerd.yaml`）。Istio と同一クラスタに入れると両方の injection webhook が競合するリスクがあるため分離する。

```bash
# Linkerd クラスタに切り替え
kubectl config use-context kind-learning-linkerd

# Linkerd の前提チェック
linkerd check --pre
```

## 動作確認

```bash
# ServiceProfile のリトライ動作確認
kubectl run -it --rm curl --image=curlimages/curl --restart=Never \
  --context kind-learning-linkerd -- \
  curl -s http://api.linkerd-app/api/v1/echo

# TrafficSplit の反映確認（v1/v2 の分散を stat で見る）
linkerd viz stat trafficsplit/api -n linkerd-app
```
