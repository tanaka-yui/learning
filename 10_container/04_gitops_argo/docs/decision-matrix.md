# Decision Matrix: Kubernetes (Argo) vs AWS マネージド

## 要件別 選択表

| 要件 | Kubernetes (Argo) | AWS マネージド | 備考 |
|---|---|---|---|
| マルチクラウド | ◎ | △ | Argo は GKE/EKS/AKS で動作。CodePipeline は AWS 専用 |
| 学習コスト | 高 | 低 | Argo は CRD + RBAC の理解が必要 |
| ロックイン | 低 | 高 | Argo は OSS。AWS は移植困難 |
| 統合 UX | △ | ◎ | AWS コンソールは一元管理。Argo は複数 UI を使い分け |
| 大規模 CI/CD | ◎ | ◎ (Enterprise) | 両者ともスケール可能。Argo は自前管理が必要 |
| 運用負荷 | 高 | 低 | コントローラの HA・バージョン管理を自前で行う |
| 既存 Kubernetes | ◎ | △ | Kubernetes 運用済みなら Argo 追加は低コスト |
| Kubernetes 非利用 | △ | ◎ | クラスタがないなら AWS の方が圧倒的に早い |
| Progressive Delivery | ◎ | △ (CodeDeploy のみ) | Rollout の canary/blueGreen が柔軟 |
| イベント駆動 CI/CD | ◎ | ◎ | Argo Events vs EventBridge どちらも強力 |
| 可視化 / デバッグ | ◎ | △ | Argo CD UI / Rollouts Dashboard が優秀 |
| コスト (小規模) | △ | ◎ | クラスタコストが固定でかかる |
| コスト (大規模) | ◎ | △ | スケールするほど Kubernetes の方がコスト効率が良い |

## 選択フローチャート

```
クラスタを既に Kubernetes で運用している?
├─ Yes → Argo CD + Argo Workflows を検討
│         マルチクラウド / Kubernetes 固有機能が必要?
│         ├─ Yes → Argo フルスタック
│         └─ No  → Argo CD のみ + AWS CI/CD 併用でも可
└─ No  → AWS マネージドサービスが最速
          将来的に Kubernetes 移行の可能性?
          ├─ 高  → 設計段階から GitOps 原則を採用しておく
          └─ 低  → AWS サービスに集中
```

## ツール別選択指針

### CD (デプロイメント)
- **Argo CD**: Kubernetes マニフェストを Git で管理したい。drift 自動修正が必要。
- **CodePipeline**: AWS ネイティブサービス (ECS/Lambda) を中心に CD を回したい。

### ワークフローエンジン
- **Argo Workflows**: 複雑な DAG / ファンアウト / アーティファクト管理が必要。Kubernetes コンテナで処理したい。
- **Step Functions**: サーバレス中心。Lambda / ECS Task の連携が主体。

### イベントトリガー
- **Argo Events**: Kubernetes 内のワークフローをイベントで起動したい。SQS/Kafka/webhook 統合が必要。
- **EventBridge**: AWS サービス間のイベントルーティング。Kinesis/SQS/Lambda との親和性が高い。

### Progressive Delivery
- **Argo Rollouts**: Kubernetes Deployment を柔軟に canary/blueGreen 化したい。カスタムメトリクス判定が必要。
- **CodeDeploy**: ECS / EC2 の blue/green デプロイ。AWS 統合が楽。

### CI ランナー
- **GHA Runner Controller**: GitHub Actions を Kubernetes 上で実行。コンテナイメージ・Kubernetes API への直接アクセスが必要。
- **CodeBuild**: AWS 統合の CI。IAM ロールと自然に組み合わせられる。完全マネージドで運用不要。

## 本章の学習成果まとめ

本章 (10-4) では以下を実装・体験した:

| ツール | 実装 | AWS 対応 |
|---|---|---|
| Argo CD | Application / ApplicationSet / AppProject | CodePipeline |
| Flux | GitRepository / Kustomization / Alert | (同上) |
| Argo Workflows | DAG / CronWorkflow / WorkflowTemplate | Step Functions |
| Argo Rollouts | canary / AnalysisTemplate | CodeDeploy |
| Argo Events | EventBus / EventSource / Sensor | EventBridge |
| ARC | AutoscalingRunnerSet / containerMode | CodeBuild |
