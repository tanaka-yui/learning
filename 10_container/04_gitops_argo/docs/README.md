# 10-4 GitOps & Argo スタック ドキュメント

## 学習順の目次

| # | ファイル | 内容 |
|---|---|---|
| 1 | [01 GitOps 基礎](./01-gitops-basics.md) | GitOps 4 原則、Push vs Pull、ドリフト検出、Sync Wave、App-of-Apps、Mono/2-Repo |
| 2 | [02 Argo CD](./02-argocd.md) | Application / ApplicationSet / AppProject / RBAC / SSO / Sync Policy / Hooks / Notifications |
| 3 | [03 Flux](./03-flux.md) | GitRepository / OCIRepository / Kustomization / HelmRelease / Alert & Provider / Image Automation、Argo CD 比較 |
| 4 | [04 Argo Workflows](./04-argo-workflows.md) | Workflow / WorkflowTemplate / CronWorkflow / DAG vs Steps / Artifact / `when` / `withItems` / RBAC gotcha |
| 5 | [05 Argo Rollouts](./05-argo-rollouts.md) | Rollout / AnalysisTemplate / canary / blueGreen / Metric Providers / Istio 連携 / CodeDeploy 対応 |
| 6 | [06 Argo Events](./06-argo-events.md) | EventBus / EventSource / Sensor / Trigger 種類 / `operate-workflow-sa` gotcha / EventBridge 対応 |
| 7 | [07 GHA Runner Controller](./07-gha-runner-controller.md) | ARC アーキテクチャ / AutoscalingRunnerSet / containerMode / PAT vs GitHub App / image.tag gotcha / CodeBuild 対応 |
| 8 | [08 AWS 対比](./08-aws-comparison.md) | Argo vs AWS 対応表 / Kubernetes vs AWS トレードオフ / LocalStack 対応状況 |
| 9 | [Decision Matrix](./decision-matrix.md) | 要件別選択表 / 選択フローチャート / ツール別指針 |

## 推奨学習順序

```
01 (基礎) → 02 (Argo CD) → 03 (Flux) → 04 (Workflows) → 05 (Rollouts)
→ 06 (Events) → 07 (ARC) → 08 (AWS 対比) → 09 (Decision Matrix)
```

GitOps の考え方 (01) を理解してから、各ツール (02-07) を順番に学ぶ。最後に AWS との対比 (08) と選択指針 (decision-matrix) で全体像を整理する。

## 主要な gotcha まとめ

| タスク | 問題 | 対処 |
|---|---|---|
| Task 4 (Workflows) | `default` SA に `workflowtaskresults` create 権限がない | `Role` + `RoleBinding` を作成、または専用 SA を使用 |
| Task 5 (Rollouts) | Argo CD と Rollout が同一リソースを競合管理 | Rollout 追加後は Argo CD 管理の Deployment を削除 |
| Task 6 (Events) | `operate-workflow-sa` が Workflow Namespace に存在しない | 両 Namespace に SA と RoleBinding を作成 |
| Task 7 (ARC) | `image.tag` 固定が chart バージョンと競合 | `image.tag` は省略して chart デフォルトを使用 |

## 実装ファイル構成

```
10_container/04_gitops_argo/
├── apps/
│   ├── argocd/        # Argo CD install values + Application/ApplicationSet/AppProject
│   ├── flux/          # Flux GitRepository / Kustomization / Notification
│   ├── workflows/     # Workflow / WorkflowTemplate / CronWorkflow
│   ├── rollouts/      # Rollout / AnalysisTemplate
│   ├── events/        # EventBus / EventSource / Sensor
│   └── gha-runner/    # AutoscalingRunnerSet / RunnerDeployment (legacy)
├── comparison/aws/    # Terraform による AWS 対比実装
├── envs/              # dev / stg / prod Kustomize overlays
└── docs/              # 本ドキュメント群
```
