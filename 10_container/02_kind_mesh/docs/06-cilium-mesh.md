# Cilium — eBPF data path・L7 NetworkPolicy・Hubble 観測

## Cilium とは

**Cilium** は eBPF (extended Berkeley Packet Filter) をデータプレーンに使う CNI プラグイン兼サービスメッシュである。
従来の iptables/ipvs を使わずに Linux カーネルの eBPF JIT コンパイラでパケットを処理するため、高スループット・低レイテンシを実現する。

## eBPF data path

```
Pod egress
  │
  ▼
TC eBPF program (veth)      ← Cilium が attach
  │  L3/L4/L7 policy 評価
  │  NAT / load balancing
  ▼
物理 NIC / overlay (VXLAN / Geneve)
```

eBPF program はカーネル空間で動くため、ユーザ空間コンテキストスイッチが発生しない。
Cilium は eBPF map（共有メモリ）にルーティングテーブル・ポリシー・接続追跡を保持する。

## kube-proxy 代替

`kind/cluster-cilium.yaml` で `kubeProxyMode: none` を設定するとクラスタに kube-proxy がデプロイされない。
Cilium が Service の ClusterIP/NodePort/LoadBalancer を eBPF で実装する。

利点:
- iptables のルール数が O(n) で増加しない（eBPF map は O(1) lookup）
- conntrack テーブルの枯渇リスクがない
- DSR (Direct Server Return) による高スループット

## Hubble — フロー観測

**Hubble** は Cilium 組み込みの観測レイヤで、Pod 間のネットワークフローを L7 まで可視化する。

```bash
# Hubble UI を port-forward で開く
cilium hubble ui &

# CLI でリアルタイムフロー確認
hubble observe --namespace cilium-app --follow

# L7 HTTP フローのみ表示
hubble observe --namespace cilium-app --protocol http
```

Hubble は Cilium の eBPF フックからフロー情報を取得するため、追加の sidecar が不要。

## CiliumNetworkPolicy — L3/L4/L7

標準の `NetworkPolicy` (L3/L4) に加え、Cilium 独自の `CiliumNetworkPolicy` で HTTP レベルのポリシーを書ける。

```yaml
# manifests/overlays/cilium/l7policy.yaml
apiVersion: cilium.io/v2
kind: CiliumNetworkPolicy
metadata:
  name: api-l7
  namespace: cilium-app
spec:
  endpointSelector:
    matchLabels:
      app: api
  ingress:
  - fromEndpoints:
    - matchLabels:
        app: web
    toPorts:
    - ports:
      - port: "8080"
        protocol: TCP
      rules:
        http:
        - method: GET
          path: "/api/v[12]/echo"
        - method: GET
          path: "/healthz"
```

このポリシーは `web` Pod からの GET のみ許可し、それ以外（POST、`/admin` など）はカーネルレベルでドロップする。

## Cluster Mesh

**Cluster Mesh** は複数の Cilium クラスタを L3 レベルで接続し、クロスクラスタの Service discovery と NetworkPolicy を実現する機能。

```bash
cilium clustermesh enable --context kind-cluster-a
cilium clustermesh enable --context kind-cluster-b
cilium clustermesh connect \
  --context kind-cluster-a \
  --destination-context kind-cluster-b
```

接続後、クラスタ A の Pod がクラスタ B の Service に ClusterIP でアクセスできる。フェイルオーバーや地理分散 Active-Active 構成に使う。

## Service Mesh モード

Cilium には CNI に加えてサービスメッシュ機能もある。

| モード | mTLS | L7 routing | sidecar |
|---|---|---|---|
| CNI のみ | なし | CiliumNetworkPolicy (L7) | 不要 |
| sidecar モード | あり | Envoy sidecar 経由 | 必要 |
| sidecar-free (Envoy per-node) | あり | ノード 1 台に Envoy | 不要 |

本チャプターでは CNI + L7 CiliumNetworkPolicy の構成を使い、サービスメッシュ機能は Istio/Linkerd と組み合わせない。

## 動作確認

```bash
kubectl config use-context kind-learning-cilium

# Cilium の動作確認
cilium status

# L7 ポリシーのテスト（許可）
kubectl run -it --rm curl --image=curlimages/curl --namespace cilium-app \
  --labels="app=web" --restart=Never -- \
  curl -s http://api:8080/api/v1/echo

# L7 ポリシーのテスト（拒否）
kubectl run -it --rm curl2 --image=curlimages/curl --namespace cilium-app \
  --labels="app=web" --restart=Never -- \
  curl -s -X POST http://api:8080/api/v1/echo
```
