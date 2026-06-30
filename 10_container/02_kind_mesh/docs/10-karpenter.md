# Karpenter (OSS) — NodePool・kwok provider・オートスケール

## Karpenter とは

**Karpenter** は AWS が開発した Kubernetes ノードオートスケーラーで、2023 年に OSS 化された。
従来の Cluster Autoscaler が AutoScaling Group のスケールインのみを操作するのに対し、Karpenter は **ノードを直接プロビジョニング** する。

| 観点 | Cluster Autoscaler | Karpenter |
|---|---|---|
| スケールアップ速度 | ~数分（ASG 経由） | **~数十秒**（直接 EC2 API 呼び出し）|
| ノード種別の柔軟性 | ASG 単位で固定 | NodePool で幅広く指定可能 |
| コスト最適化 | 手動設定 | Spot / On-demand の自動混在 |
| Bin packing | 弱い | **強い（Pending Pod から最適ノードを計算）**|

## NodePool と NodeClass

Karpenter の設定は 2 つのリソースで行う。

```yaml
# manifests/overlays/karpenter/nodepool.yaml（抜粋）
apiVersion: karpenter.sh/v1
kind: NodePool
metadata:
  name: kwok-default
spec:
  template:
    spec:
      requirements:
      - key: kubernetes.io/os
        operator: In
        values: [linux]
      - key: karpenter.sh/capacity-type
        operator: In
        values: [on-demand]
      nodeClassRef:
        group: karpenter.kwok.sh
        kind: KWOKNodeClass
        name: default
  limits:
    cpu: "100"
  disruption:
    consolidationPolicy: WhenEmpty
    consolidateAfter: 30s
```

| リソース | 役割 |
|---|---|
| `NodePool` | ノードの要件（OS・CPU アーキテクチャ・Spot 可否）と上限を定義 |
| `NodeClass` | クラウドプロバイダ固有の設定（AMI ID・subnet・security group 等）|

## kwok provider — ローカル学習

**kwok (Kubernetes WithOut Kubelet)** はノードをシミュレートするツールで、実際の VM/コンテナを起動せずにノードを追加できる。これを Karpenter の provider として使うことで、kind 上でオートスケールの挙動を学習できる。

> **重要 (2026 年中頃時点)**: `awslabs/karpenter-provider-kwok` リポジトリは**アーカイブ／削除**されており、アップストリームでのメンテナンスが終了している。
> そのため kwok provider を公式リポジトリから取得できない状態にある。

**代替手段**:

1. **cluster-api-provider-docker (CAPD)**: Cluster API の Docker provider でローカル kind ベースのノード追加をシミュレートできる。Karpenter 本体の動作理解には適している。

2. **AWS Free Tier で直接テスト**: `t3.micro` 1 台の EKS クラスタを作成し、Karpenter の AWS EC2 Provider (`karpenter.k8s.aws/v1`) を実際に使う。Free Tier 範囲内で動作確認可能。

3. **fork 版を使う**: コミュニティが維持する fork（例: `kubernetes-sigs/karpenter-provider-kwok`）が存在する場合はそちらを参照する。

## AWS EC2 Provider への展開

本番環境では `EC2NodeClass` を使う。

```yaml
apiVersion: karpenter.k8s.aws/v1
kind: EC2NodeClass
metadata:
  name: default
spec:
  amiFamily: AL2023
  role: "KarpenterNodeRole-${CLUSTER_NAME}"
  subnetSelectorTerms:
    - tags:
        karpenter.sh/discovery: "${CLUSTER_NAME}"
  securityGroupSelectorTerms:
    - tags:
        karpenter.sh/discovery: "${CLUSTER_NAME}"
```

## オートスケールの動作

`manifests/overlays/karpenter/load.yaml` は Pending Pod を大量に作成してスケールアップをトリガーする。

```yaml
# replicas: 20、各 Pod が cpu: 500m をリクエスト → 計 10 CPU 分の Pending
```

Karpenter は Pending Pod の要求をまとめて最適なノードサイズを計算し、1 台のノードを起動して全 Pod を収容する（Bin packing）。

## ECS Capacity Provider との対応

| AWS サービス | コンセプト | Karpenter 相当 |
|---|---|---|
| ECS Capacity Provider | タスク数に応じて EC2 をスケール | NodePool + NodeClass |
| ECS タスク定義 | コンテナの実行単位 | Pod spec |
| ECS サービス | 望ましいタスク数を維持 | Deployment + HPA |

ECS では Capacity Provider がノードを管理するが、Kubernetes では Karpenter が同等の役割を担う。詳細は `10_container/03_ecs/` を参照。

## 動作確認（kwok provider 使用時）

```bash
# 負荷 Deployment を apply して Pending を発生させる
kubectl apply -k manifests/overlays/karpenter/

# Karpenter が NodeClaim を作成してノードが追加されることを確認
kubectl get nodeclaim
kubectl get nodes

# Pending Pod が解消されることを確認
kubectl get pods -n karpenter-demo -w
```
