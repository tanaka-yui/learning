# 10-2 kind + 最新コンテナエコシステム

| 技術 | 学習対象 | overlay |
|---|---|---|
| Istio (sidecar/ambient) | mTLS, traffic shift, waypoint | istio-sidecar / istio-ambient |
| Envoy Gateway | Gateway API | envoy-gateway |
| Linkerd | Rust micro-proxy, SMI | linkerd |
| Cilium | eBPF, L7 NetworkPolicy, Hubble | cilium |
| SpinKube | WASM workload | spinkube |
| Kata/gVisor | Sandbox runtime, RuntimeClass | kata |
| vCluster | 仮想クラスタ | vcluster |
| Karpenter (OSS) | NodePool / kwok provider | karpenter |

検証: `make verify`。詳細: `docs/`, `VERIFICATION.md`。
