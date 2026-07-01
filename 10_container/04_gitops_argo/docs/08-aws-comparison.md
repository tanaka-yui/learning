# 08 AWS 対比

実装参照: `comparison/aws/`

## 対応表

| Kubernetes (Argo スタック) | AWS マネージドサービス | LocalStack 対応 |
|---|---|---|
| Argo CD Application | CodePipeline (Source → Build → Deploy) | 非対応 (501) |
| ApplicationSet | CodePipeline + 複数パイプライン | 非対応 (501) |
| Argo Workflows DAG | Step Functions (Parallel/Map/Choice) | Community OK |
| Argo Events EventSource | EventBridge Rule / API Destination | Community OK |
| Argo Events Sensor | EventBridge Rule + Target | Community OK |
| Argo Rollouts canary | CodeDeploy canary (Deployment Group) | 非対応 (501) |
| Argo Rollouts blueGreen | CodeDeploy blue/green | 非対応 (501) |
| Rollout AnalysisTemplate | CodeDeploy Lifecycle Hook | 非対応 (501) |
| GHA Runner Controller | CodeBuild (self-hosted は不要) | 非対応 (501) |

LocalStack Community で動作確認できるのは Step Functions と EventBridge のみ。CodePipeline / CodeBuild / CodeDeploy は Community 非対応 (HTTP 501) で、本物 AWS 環境が必要。

## Terraform ファイル

`comparison/aws/terraform/` 以下:

- `codepipeline.tf`: CodePipeline + S3 アーティファクトストア (count=0 で default 無効、apply 失敗回避)
- `stepfunctions.tf`: Step Functions State Machine (Parallel state で Argo Workflows DAG に対応)
- `eventbridge.tf`: EventBridge Rule + Target (Argo Events Sensor/Trigger に対応)
- `variables.tf`, `outputs.tf`, `providers.tf`: 共通設定

## Kubernetes vs AWS の選択トレードオフ

### Kubernetes (Argo スタック) の強み

| 観点 | 内容 |
|---|---|
| 選択の自由 | CD / Workflow / ProgressiveDelivery / CI を個別に選択・組み合わせ可能 |
| エコシステム | CNCF エコシステムが豊富。Helm / Kustomize / Istio と自然に統合できる |
| マルチクラウド | GKE / EKS / AKS 上で同じマニフェストが動く |
| ロックイン低 | 特定クラウドベンダーに縛られない |
| 可視化 | Argo CD UI / Rollouts Dashboard でデプロイ状態を直感的に確認 |

### Kubernetes (Argo スタック) の課題

| 観点 | 内容 |
|---|---|
| 運用負荷 | Argo CD / Workflows / Rollouts / Events / ARC それぞれを自前で管理 |
| 学習コスト | CRD の数が多く、RBAC 設定も複雑になりがち |
| 可用性 | コントローラが落ちると CD が止まる |

### AWS マネージドの強み

| 観点 | 内容 |
|---|---|
| 統合 UX | CodePipeline / CodeBuild / CodeDeploy がコンソールで一元管理 |
| 運用負荷低 | インフラ管理不要。HA / スケールが自動 |
| IAM 統合 | AWS の細粒度 IAM がそのまま使える |
| サポート | AWS Enterprise Support で 24/7 対応可能 |

### AWS マネージドの課題

| 観点 | 内容 |
|---|---|
| ロックイン | CodePipeline は AWS 専用。他クラウドへの移植は困難 |
| Kubernetes 連携 | Kubernetes 固有機能 (Rollout, SyncWave) は別途 Argo 等が必要 |
| 柔軟性 | パイプラインの構成がサービス仕様に縛られる |

## 学習指針

```
小規模 / AWS ネイティブ構成 → CodePipeline + CodeDeploy + EventBridge が早い
規模拡大 / マルチクラウド / Kubernetes 中心 → Argo スタックへ移行
```

Kubernetes クラスタを既に運用している場合、Argo CD を追加するコストは低く、CodePipeline との併存も可能。両者は排他ではなく、CI は GitHub Actions (ARC)、CD は Argo CD、というハイブリッドが現実的な構成となる。
