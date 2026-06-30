# Workload Mapping: ECS vs Kubernetes

## 概念対応表

| 概念 | ECS | Kubernetes |
|---|---|---|
| 実行単位 | Task (1 つ以上の container) | Pod (1 つ以上の container) |
| 常駐ワークロード | Service (replica / daemon) | Deployment / DaemonSet / StatefulSet |
| 単発実行 | RunTask (API / EventBridge) | Job |
| 定期実行 | Scheduled Task (EventBridge Rules) | CronJob |
| クラスタ | ECS Cluster | k8s Cluster (Control Plane + Node) |
| 配置制御 | Capacity Provider (FARGATE / FARGATE_SPOT / EC2 ASG) | NodeSelector / Affinity / Taint+Toleration |
| サイドカー | Task 定義内で container を並列宣言 | sidecar container / native sidecar (k8s 1.28+) |
| 初期化処理 | `dependsOn` + `healthCheck` による起動順制御 | initContainer (順序保証) |

## Task / Pod ライフサイクル

**ECS Task**

```
PROVISIONING → PENDING → ACTIVATING → RUNNING → DEACTIVATING → STOPPING → DEPROVISIONING → STOPPED
```

- `PROVISIONING`: Fargate の場合はコンテナ実行環境の確保 (cold start)
- `PENDING`: image pull + ネットワーク設定
- `RUNNING`: healthCheck HEALTHY になるまで Service は in-service 扱いしない

**Kubernetes Pod**

```
Pending → Running → Succeeded / Failed / Unknown
(内部フェーズ: initContainers → containers; readinessGate 通過後に Endpoints に追加)
```

- `Pending`: スケジューリング待ち + image pull
- `Running`: 全 container が起動済み。readinessProbe が PASS するまで Service の Endpoint から除外

## CRUD CLI コマンド対比

| 操作 | ECS (AWS CLI) | Kubernetes (kubectl) |
|---|---|---|
| 一覧取得 | `aws ecs list-tasks --cluster <c>` | `kubectl get pods` |
| 詳細確認 | `aws ecs describe-tasks ...` | `kubectl describe pod <name>` |
| ログ確認 | `aws logs get-log-events ...` | `kubectl logs <pod>` |
| 手動起動 | `aws ecs run-task ...` | `kubectl run <name> --image=...` |
| スケール | `aws ecs update-service --desired-count N` | `kubectl scale deploy/<name> --replicas=N` |
| 削除 | `aws ecs delete-service --force` | `kubectl delete deploy/<name>` |

## LocalStack 制限事項

> LocalStack Community では ECS Task の一部ライフサイクル遷移 (例: `PROVISIONING` 状態の保持) が省略されます。
> `make verify` の動作範囲と Pro 機能の差分については [VERIFICATION.md](../VERIFICATION.md) を参照してください。
