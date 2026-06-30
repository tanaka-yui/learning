# Network: ECS vs Kubernetes

## 概念対応表

| 概念 | ECS | Kubernetes |
|---|---|---|
| Task / Pod のネットワーク | `awsvpc` (推奨) / `bridge` / `host` | CNI プラグイン抽象 (Flannel / Calico / Cilium 等) |
| 内部サービス発見 | AWS Cloud Map + Service Discovery | Service (ClusterIP) + CoreDNS |
| L4 公開 | NLB (target=IP, awsvpc mode) | `Service type=LoadBalancer` |
| L7 公開 | ALB (target=IP) ※LocalStack Pro 限定 | Ingress / Gateway API |
| 内部サービスメッシュ | Service Connect (App Mesh は廃止予定) | Istio / Linkerd / Cilium Service Mesh ([→10-2](../../02_service_mesh/)) |
| ネットワークポリシー | Security Group (ENI 単位) | `NetworkPolicy` / `CiliumNetworkPolicy` |

## awsvpc モードと ENI

`awsvpc` は Task ごとに ENI を割り当てる。Task から見ると VPC の 1st class citizen として扱われ、SG を直接アタッチできる。

```
VPC
└── Subnet
    ├── ENI (Task-A) ←── Security Group A
    └── ENI (Task-B) ←── Security Group B
```

**制約**: インスタンスタイプごとに ENI 数の上限があるため、EC2 launch type では密度が制限される。Fargate は 1 Task = 1 ENI で上限の問題が起きにくい。

## Kubernetes CNI 抽象

k8s はネットワーク実装を CNI プラグインに委譲する。Pod は起動時に CNI から IP を払い出される。

```
kubelet
  └── CNI plugin (例: Cilium)
        ├── Pod IP 割当 (IPAM)
        ├── eBPF datapath
        └── NetworkPolicy 強制
```

クラウド側では `aws-node` (AWS VPC CNI) を使うと Pod に VPC IP が割り当てられ、ECS awsvpc と同等のルーティングが実現できる。

## Service Connect の概念

ECS Service Connect はサイドカープロキシ (Envoy) を自動注入し、サービス間通信に論理名前 (`http://backend:8080`) でアクセスできるようにする。

```
[frontend Task]
  └── envoy sidecar ──→ (Service Connect 名前解決) ──→ [backend Task]
                                                          └── envoy sidecar
```

k8s 側の相当機能は Istio / Cilium のサービスメッシュ。詳細は [10-2 サービスメッシュ](../../02_service_mesh/) を参照。

## LocalStack 制限事項

ALB ターゲットグループ (L7 公開) は LocalStack Pro が必要。Community 版では NLB までが検証範囲となる。詳細は [VERIFICATION.md](../VERIFICATION.md) を参照。
