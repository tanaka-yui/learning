# 意思決定マトリクス: ECS vs Kubernetes

## 要件別マトリクス

| 要件 | ECS 有利 | k8s 有利 | 備考 |
|---|---|---|---|
| AWS 専用・マネージド優先 | ◎ | △ | ECS は AWS サービスとの統合が深い |
| マルチクラウド / ポータビリティ | △ | ◎ | k8s は CNCF 標準、クラウド間移植が容易 |
| 高度なサービスメッシュ / Operator | △ | ◎ | Istio / Cilium 等のエコシステムが豊富 |
| サーバレス重視 | ◎ Fargate | ◎ Autopilot / Karpenter | どちらもサーバレス選択肢あり |
| 学習コスト | 低 | 高 | ECS は概念が少なく AWS 経験者に馴染みやすい |
| 運用コスト | 中 | 中〜高 | k8s はコントロールプレーン更新・アドオン管理が必要 |
| ステートフルワークロード | △ | ◎ | StatefulSet / PVC / StorageClass が充実 |
| バッチ / 大規模ジョブ | ○ | ◎ | k8s Job / IndexedJob / KEDA が強力 |
| GPU / 特殊ハードウェア | △ (EC2 launch type) | ◎ | DevicePlugin で多様なアクセラレータに対応 |
| セキュリティポリシー細粒度 | ○ (SG + Task Role) | ◎ | NetworkPolicy / OPA / Kyverno 等が豊富 |
| チーム k8s 習熟度あり | — | ◎ | 習熟済みであれば k8s の複雑さはペナルティにならない |

## 判断フロー

```
AWS のみで完結し、k8s エコシステムが不要か？
  ├─ YES → ECS (Fargate / EC2)
  │          └─ 複雑なスケジューリングや Operator が必要になったら EKS に移行検討
  └─ NO  → Kubernetes (EKS / GKE / AKS 等)
              ├─ ノード管理ゼロ優先 → EKS Fargate / EKS Autopilot
              └─ コスト最適化・DaemonSet・GPU → Karpenter + EC2
```

## 5 軸サマリ

| 軸 | ECS | k8s | 詳細 |
|---|---|---|---|
| Workload | Task / Service / Job | Pod / Deployment / Job / StatefulSet | [→01](01-workload-mapping.md) |
| Network | awsvpc + SG + Service Connect | CNI + NetworkPolicy + Ingress/Mesh | [→02](02-network.md) |
| Storage | EFS / EBS (限定) | StorageClass + CSI (豊富) | [→03](03-storage.md) |
| Auth + Secrets | Task Role + SecretsManager | IRSA + ESO | [→04](04-auth-secrets.md) |
| Autoscale | Service AS + Capacity Provider | HPA + VPA + Karpenter + KEDA | [→05](05-autoscale.md) |
| Fargate vs Node | Fargate / FARGATE_SPOT | EKS Fargate / Autopilot / Karpenter | [→06](06-fargate-vs-node.md) |

## 結論

- **ECS を選ぶとき**: AWS ファーストで素早く本番稼働させたい、チームが IAM / CloudWatch に慣れている、運用チームが小さい
- **k8s を選ぶとき**: マルチクラウド対応が必要、高度な Operator エコシステムを活用したい、将来的なポータビリティを確保したい、チームに k8s 習熟者がいる
- **どちらでも解決できる**: サーバレスコンテナ実行、EFS 共有ストレージ、SecretsManager 統合
