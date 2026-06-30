# Istio サイドカーモード — mTLS・VirtualService canary・Gateway

## アーキテクチャ概観

Istio のサイドカーモードは **data plane** と **control plane** の 2 層で構成される。

| レイヤ | コンポーネント | 役割 |
|---|---|---|
| data plane | Envoy sidecar | 各 Pod に自動注入。実トラフィックを横取りして mTLS 確立・テレメトリ収集・ルーティング適用 |
| control plane | istiod | 証明書発行 (CA)・xDS config 配信・Webhook で sidecar 注入 |

## sidecar 自動注入

`istiod` が MutatingWebhookConfiguration を登録しており、Namespace に `istio-injection: enabled` ラベルがあると Pod 作成時に `istio-proxy` (Envoy) コンテナと `istio-init` init コンテナが自動追加される。

```bash
kubectl label namespace istio-sidecar-app istio-injection=enabled
```

`istio-init` は Pod の iptables ルールを書き換えてすべての inbound/outbound トラフィックを Envoy にリダイレクトする。

## PeerAuthentication — mTLS の強制

```yaml
# manifests/overlays/istio-sidecar/peerauth.yaml
apiVersion: security.istio.io/v1
kind: PeerAuthentication
metadata: { name: default }
spec: { mtls: { mode: STRICT } }
```

`STRICT` を設定すると Namespace 内の全 Service がプレーンテキストを拒否し、相互 TLS のみ許可する。証明書は istiod が SPIFFE/SVID として自動発行・ローテーションするため、アプリ側の変更は不要。

## VirtualService と DestinationRule — カナリア

```yaml
# manifests/overlays/istio-sidecar/virtualservice.yaml
apiVersion: networking.istio.io/v1
kind: VirtualService
metadata: { name: api }
spec:
  hosts: ["api"]
  http:
  - route:
    - destination: { host: api, subset: v1 }
      weight: 90
    - destination: { host: api, subset: v2 }
      weight: 10
```

`weight` の合計は 100 でなければならない。`subset` は DestinationRule で定義する。

```yaml
# manifests/overlays/istio-sidecar/destinationrule.yaml
apiVersion: networking.istio.io/v1
kind: DestinationRule
metadata: { name: api }
spec:
  host: api
  subsets:
  - name: v1
    labels: { version: v1 }
  - name: v2
    labels: { version: v2 }
```

`v1`/`v2` のラベルはそれぞれ `manifests/base/api-v1.yaml` と `api-v2.yaml` の Pod に付与されている。`weight` を変えるだけでトラフィック比率を変更できる。

## Gateway 経由の外部公開

クラスタ外からアクセスさせるには Istio `Gateway` と `VirtualService` を組み合わせる。

```yaml
# manifests/overlays/istio-sidecar/gateway.yaml（抜粋）
apiVersion: networking.istio.io/v1
kind: Gateway
metadata: { name: api-gw }
spec:
  selector: { istio: ingressgateway }
  servers:
  - port: { number: 80, name: http, protocol: HTTP }
    hosts: ["sidecar.example"]
---
apiVersion: networking.istio.io/v1
kind: VirtualService
metadata: { name: api-ext }
spec:
  hosts: ["sidecar.example"]
  gateways: ["api-gw"]
  http:
  - route:
    - destination: { host: api, subset: v1 }
      weight: 90
    - destination: { host: api, subset: v2 }
      weight: 10
```

`kind/cluster.yaml` の `extraPortMappings` (port 80→80) によりホストから `http://sidecar.example` にアクセスできる（`/etc/hosts` に `127.0.0.1 sidecar.example` を追記する）。

## Kiali による観測

Kiali は istiod と連携してサービスグラフ・トラフィックフロー・メトリクスを可視化する。

```bash
kubectl port-forward svc/kiali -n istio-system 20001:20001
# ブラウザで http://localhost:20001 を開く
```

Graph ビューでは VirtualService の weight がリアルタイムに反映され、v1/v2 の振り分けを視覚的に確認できる。エラーレート・レイテンシの異常も色で即座に把握できる。

## 動作確認

```bash
# カナリア比率の確認（10 回リクエストして v2 が約 1 回返ること）
for i in $(seq 1 10); do
  curl -s -H "Host: sidecar.example" http://localhost/api/v1/echo | grep version
done
```
