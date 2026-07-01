# 04 Argo Workflows

実装参照: `apps/workflows/`

## 主要リソース

### Workflow

単発のワークフロー実行単位。`entrypoint` に開始テンプレートを指定する。`generateName` を使うと実行ごとにユニークな名前が付く。

```yaml
# apps/workflows/ci-build-promote.yaml (抜粋)
kind: Workflow
spec:
  entrypoint: main
  templates:
  - name: main
    dag:
      tasks:
      - name: git-clone
        template: echo
      - name: build
        dependencies: [git-clone]
        template: echo
      - name: scan
        dependencies: [build]
        template: echo
      - name: promote
        dependencies: [scan]
        template: echo
```

### WorkflowTemplate

再利用可能なテンプレートを Namespace スコープで定義する。`templateRef` で他 Workflow から参照できる。本章では `shared` WorkflowTemplate に `git-echo` テンプレートを定義している (`apps/workflows/workflowtemplate-shared.yaml`)。

### CronWorkflow

cron 式でワークフローを定期実行する。`concurrencyPolicy: Forbid` で同一 cron の多重起動を防げる。本章の `drift-report` CronWorkflow は 1 時間ごとにドリフト確認を実行する (`apps/workflows/cron-report.yaml`)。

### ClusterWorkflowTemplate

WorkflowTemplate のクラスタスコープ版。複数 Namespace から参照できるため、組織共通のビルドテンプレートなどに使う。

## DAG vs Steps

| | DAG | Steps |
|---|---|---|
| 記述 | `dag.tasks[].dependencies` で並列関係を宣言 | `steps[][]` でシーケンシャルなステージを定義 |
| 並列 | 依存なし task は自動並列実行 | 同一ステージ内の複数 step が並列実行 |
| 可読性 | 複雑な依存関係が見やすい | 直列フローが分かりやすい |

本章の `ci-build-promote.yaml` は DAG を使い `git-clone → build → scan → promote` の依存チェーンを表現している。

## Artifact と Parameter

- **Parameter**: 文字列の入出力。`inputs.parameters` / `outputs.parameters` で次 step に値を渡せる。
- **Artifact**: ファイルの入出力。S3 / GCS / MinIO などのオブジェクトストレージと連携して中間成果物を保存・受け渡しする。

## `when` 条件分岐

task / step に `when: "{{inputs.parameters.env}} == prod"` を付けると条件付き実行ができる。false の場合はその task がスキップされる。

## `withItems` ファンアウト

```yaml
- name: test-matrix
  template: unit-test
  withItems: [go1.21, go1.22, go1.23]
  arguments:
    parameters:
    - { name: version, value: "{{item}}" }
```

`withItems` でリストの各要素に対して同じ template を並列実行できる。マトリックステストや複数環境へのデプロイに使う。

## `templateRef` による資材再利用

```yaml
- name: build
  templateRef:
    name: shared          # WorkflowTemplate 名
    template: git-echo    # テンプレート名
```

`ClusterWorkflowTemplate` を参照する場合は `clusterScope: true` を追加する。

## UI と CLI

- **Argo Workflows UI**: `http://localhost:2746` (port-forward 後)。DAG の進行状況・ログ・再実行が可能。
- **argo CLI**: `argo submit`, `argo get`, `argo logs`, `argo delete` など。

## RBAC 注意点 (Task 4 gotcha)

`default` ServiceAccount で Workflow を実行すると、`workflowtaskresults` リソースへの create 権限が不足してタスク結果が保存できずエラーになる。以下で権限を付与する:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata: { name: workflow-sa-role, namespace: argo }
rules:
- apiGroups: [argoproj.io]
  resources: [workflowtaskresults]
  verbs: [create, patch]
---
kind: RoleBinding
# subjects: ServiceAccount/default を bind
```

または Helm values で `workflow.serviceAccount.create: true` を有効にして専用 SA を使うことを推奨する。

## Step Functions との対応

| Argo Workflows | AWS Step Functions |
|---|---|
| DAG template | Parallel / Map state |
| Steps template | States の直列チェーン |
| `when` | Choice state |
| `withItems` | Map state |
| Artifact (S3) | S3 連携 (ResultPath) |
| CronWorkflow | EventBridge Scheduler |
| WorkflowTemplate | Step Functions の再利用は Activity 経由 |

LocalStack Community では Step Functions がサポートされており、Terraform で state machine を定義・実行確認ができる (`comparison/aws/terraform/stepfunctions.tf`)。
