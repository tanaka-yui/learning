# Storage: ECS vs Kubernetes

## 概念対応表

| 概念 | ECS | Kubernetes |
|---|---|---|
| エフェメラルストレージ | Task エフェメラル (Fargate: 20〜200 GiB) | `emptyDir` (Pod 削除で消える) |
| Host バインド | `volumes` bindMount (EC2 launch type) | `hostPath` |
| 共有ファイルシステム | EFS volume マウント | NFS PV / EFS CSI Driver |
| ブロックストレージ | EBS アタッチメント (EC2 のみ) | EBS CSI Driver + StatefulSet |
| 動的プロビジョニング | 限定的 (EFS AP を手動作成) | `StorageClass` + CSI プラグインで自動 PV 払出 |

## ストレージライフサイクル

### ECS 側

| ストレージ種別 | Task 停止時 | Cluster 削除時 |
|---|---|---|
| エフェメラル | **削除** | 削除 |
| EFS マウント | **保持** (EFS 側に永続) | 保持 |
| EBS アタッチ | デタッチ (データ保持) | デタッチ (データ保持) |

Fargate でエフェメラル容量を増やすには Task 定義の `ephemeralStorage.sizeInGiB` を指定する (デフォルト 20 GiB、最大 200 GiB)。

### Kubernetes 側

| ストレージ種別 | Pod 削除時 | PVC 削除時 |
|---|---|---|
| `emptyDir` | **削除** | — |
| `hostPath` | **保持** (Node 上に残る) | — |
| PV (ReclaimPolicy=Retain) | **保持** | 保持 |
| PV (ReclaimPolicy=Delete) | 保持 | **削除** |

`StatefulSet` は Pod 再起動時も同一 PVC を再利用するため、ステートフルアプリに適している。

## 動的プロビジョニングの違い

ECS では EFS アクセスポイントを事前に作成し、Task 定義に `efsVolumeConfiguration` を記述する。一方 k8s の `StorageClass` + CSI は PVC を apply するだけで PV が自動払い出されるため、開発者体験が高い。

```yaml
# k8s: StorageClass で EBS を動的プロビジョニング
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: ebs-gp3
provisioner: ebs.csi.aws.com
parameters:
  type: gp3
reclaimPolicy: Delete
```

## Backup 連携

- **EFS**: AWS Backup で自動スナップショット。ECS / k8s どちらからも同一 EFS を使えるため、バックアップ設定は共通。
- **EBS**: EBS スナップショット (AWS Backup または手動)。k8s 側は `VolumeSnapshotClass` + `VolumeSnapshot` リソースで宣言的にスナップショットを管理できる。
