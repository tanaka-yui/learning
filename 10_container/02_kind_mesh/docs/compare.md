# 技術比較マトリクス — mesh / runtime / autoscaler

## サービスメッシュ機能比較

| 機能 | Istio sidecar | Istio ambient | Linkerd | Cilium Mesh |
|---|---|---|---|---|
| **mTLS** | ◎ STRICT/PERMISSIVE | ◎ STRICT (ztunnel) | ◎ 自動 | ○ Cilium WireGuard |
| **L7 routing** | ◎ VirtualService | ○ waypoint 必要 | ○ ServiceProfile | ○ CiliumNetworkPolicy |
| **カナリア** | ◎ weight 指定 | ○ waypoint 経由 | ○ TrafficSplit (SMI) | △ L7 policy のみ |
| **observability** | ◎ Kiali + Jaeger | ◎ Kiali + Jaeger | ○ linkerd viz | ◎ Hubble UI |
| **sidecar 要否** | 必要 | **不要** | 必要 (ultra-light) | **不要** |
| **footprint (idle)** | ~50–60 MB/Pod | ~ztunnel DaemonSet のみ | ~20 MB/Pod | ~eBPF program のみ |
| **学習コスト** | 高 (CRD 多数) | 中 (waypoint label 注意) | 低 (CLI 中心) | 中 (eBPF 知識必要) |
| **Gateway API** | 独自 CRD | Gateway API 準拠 | 非対応 | 対応 |

### 選択指針

- **小規模・シンプル**: Linkerd（軽量・運用コスト低）
- **豊富な traffic management + observability**: Istio sidecar
- **Pod 改修なし・大規模**: Istio ambient
- **CNI 統合・eBPF パフォーマンス**: Cilium

---

## コンテナランタイム比較

| 観点 | runc (標準) | gVisor (runsc) | Kata Containers | SpinKube (WASM) |
|---|---|---|---|---|
| **隔離レベル** | namespace/cgroup | user-space kernel | 軽量 VM | wasm sandbox |
| **起動時間** | ~50 ms | ~100 ms | ~500 ms–1 s | **~5–20 ms** |
| **メモリ (idle)** | ベースライン | +15 MB (Sentry) | +64–256 MB (VM) | **~5–10 MB** |
| **syscall 互換性** | 完全 | 一部未実装あり | ほぼ完全 | WASI サブセット |
| **kind で動作** | ◎ | △ (制限あり) | ✗ (nested virt 必要) | △ (shim 追加必要) |
| **multi-tenant 安全性** | ✗ | ○ | ◎ | ○ (wasm sandbox) |
| **主用途** | 汎用 | セキュリティ強化 | multi-tenant SaaS | イベント駆動・polyglot |

### K8s 1.33+ の注意事項

RuntimeClass が存在しない `runtimeClassName` を指定した Pod は **apply 時点で REJECT** される（Pending にならない）。事前に RuntimeClass をインストールしておくこと。

---

## オートスケーラー対応表

| ツール | スケール対象 | トリガー | プロバイダ依存 |
|---|---|---|---|
| **HPA** (Horizontal Pod Autoscaler) | Pod replicas | CPU / メモリ / カスタムメトリクス | なし |
| **VPA** (Vertical Pod Autoscaler) | Pod resource requests | CPU / メモリ使用量 | なし |
| **KEDA** | Pod replicas | イベントソース (Kafka, SQS 等) | ソース依存 |
| **Cluster Autoscaler** | Node 数 | Pending Pod | ASG 依存 |
| **Karpenter** | Node 数 (直接) | Pending Pod | AWS / KWOK |

### Karpenter vs Cluster Autoscaler

| 観点 | Cluster Autoscaler | Karpenter |
|---|---|---|
| スケールアップ速度 | ~数分 | ~数十秒 |
| ノード種別の柔軟性 | ASG 単位 | NodePool requirements で多様 |
| コスト最適化 | 手動設定 | Spot + On-demand 自動選択 |
| Bin packing | 弱い | 強い（Pending Pod から最適計算）|

---

## チャプター全体 overlay 対応表

| overlay | クラスタ | namespace | 主要リソース |
|---|---|---|---|
| istio-sidecar | learning-base | istio-sidecar-app | VirtualService, DestinationRule, PeerAuthentication, Gateway |
| istio-ambient | learning-base | istio-ambient-app | waypoint (Gateway), AuthorizationPolicy |
| envoy-gateway | learning-base | envoy-gw-app | GatewayClass, Gateway, HTTPRoute |
| linkerd | learning-linkerd | linkerd-app | ServiceProfile, TrafficSplit |
| cilium | learning-cilium | cilium-app | CiliumNetworkPolicy |
| spinkube | learning-base | spinkube-app | SpinApp |
| kata | learning-base | kata-app | RuntimeClass, Deployment (runtimeClassName: kata) |
| vcluster | learning-base | vc-a, vc-b | vCluster Helm release |
| karpenter | learning-base | karpenter-demo | NodePool, KWOKNodeClass, Deployment (load) |
