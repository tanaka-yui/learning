# Envoy Gateway — Gateway API (GatewayClass / Gateway / HTTPRoute)

## Gateway API とは

**Gateway API** は Kubernetes の Ingress を置き換えるべく SIG Network が策定した標準 API である。
Ingress が単一リソースに詰め込んでいたルーティング設定を、ロール別の複数リソースに分割している。

| リソース | 管理者 | 役割 |
|---|---|---|
| `GatewayClass` | インフラ管理者 | どの実装（controller）を使うかを宣言 |
| `Gateway` | クラスタ運用者 | リスナー（port/protocol/TLS）を定義 |
| `HTTPRoute` | アプリ開発者 | HTTP ルーティングルールを定義 |
| `TCPRoute` | アプリ開発者 | TCP レベルのルーティング |
| `TLSRoute` | アプリ開発者 | TLS passthrough ルーティング |
| `GRPCRoute` | アプリ開発者 | gRPC サービスへのルーティング |

Ingress との最大の違いは **ロール分離**（GatewayClass/Gateway はインフラ担当、HTTPRoute はアプリ担当）と **クロス Namespace ルーティング** への対応である。

## Ingress からの移行ポイント

| Ingress アノテーション | Gateway API 相当 |
|---|---|
| `nginx.ingress.kubernetes.io/rewrite-target` | `HTTPRoute.spec.rules[].filters[].urlRewrite` |
| `nginx.ingress.kubernetes.io/backend-protocol: HTTPS` | `BackendTLSPolicy` |
| `cert-manager.io/cluster-issuer` | `Gateway.spec.listeners[].tls.certificateRefs` |
| `nginx.ingress.kubernetes.io/canary-weight` | `HTTPRoute` の `backendRefs[].weight` |

## Envoy Gateway の実装位置

Envoy Gateway は Gateway API の実装の一つで、バックエンドに Envoy proxy を使う。

```
外部トラフィック
      │
      ▼
Envoy Gateway Pod (envoy-gateway-system Namespace)
      │  GatewayClass "envoy" を watch
      │  Gateway / HTTPRoute を xDS に変換
      ▼
Envoy proxy Pod (アプリ Namespace に自動生成)
      │
      ▼
backend Service (api / web)
```

Envoy Gateway 自体は controller として動作し、`Gateway` リソースを作成すると対応する Envoy proxy Pod を自動でプロビジョニングする。

## マニフェスト例

```yaml
# manifests/overlays/envoy-gateway/gateway.yaml
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata: { name: api-gw, namespace: envoy-gw-app }
spec:
  gatewayClassName: envoy
  listeners:
  - name: http
    port: 80
    protocol: HTTP
    hostname: gw.example
```

```yaml
# manifests/overlays/envoy-gateway/httproute.yaml
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata: { name: api, namespace: envoy-gw-app }
spec:
  parentRefs: [{ name: api-gw }]
  hostnames: ["gw.example"]
  rules:
  - matches: [{ path: { type: PathPrefix, value: /api } }]
    backendRefs: [{ name: api, port: 80 }]
```

`parentRefs` で `Gateway` に紐付け、`matches` でパスプレフィクスを指定する。カナリアは `backendRefs` に複数エントリを書き `weight` を設定する。

## Istio Gateway との比較

| 観点 | Istio Gateway + VirtualService | Envoy Gateway + HTTPRoute |
|---|---|---|
| 標準性 | Istio 独自 CRD | Kubernetes 標準 Gateway API |
| 移植性 | Istio が必要 | 実装非依存（Cilium Gateway / Contour も同じ API） |
| 高度なルーティング | VirtualService で細かく制御 | HTTPRoute filters で対応 |
| mTLS | PeerAuthentication と連携 | BackendTLSPolicy で管理 |
| 学習コスト | Istio 固有の概念が多い | Gateway API は汎用標準 |

本チャプターでは `learning-base` クラスタの Istio とは**別 Namespace** (`envoy-gw-app`) に Envoy Gateway をデプロイして共存させている。

## 動作確認

```bash
# Gateway に割り当てられた外部 IP を確認
kubectl get gateway api-gw -n envoy-gw-app

# /api パス経由でアクセス
curl -H "Host: gw.example" http://localhost/api/v1/echo
```
