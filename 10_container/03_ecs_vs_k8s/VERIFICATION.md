# 10-3 ECS vs k8s — 検証手順

## 前提条件

| ツール | バージョン目安 |
|---|---|
| Docker | 24+ |
| AWS CLI v2 | 2.15+ |
| terraform | 1.9+ |
| kind | v0.24+ |
| kubectl | 1.30+ |
| jq | 1.6+ |

環境変数は不要（LocalStack エンドポイントは Makefile 内で `http://localhost:4566` に固定）。

## 一括実行

```sh
cd 10_container/03_ecs_vs_k8s
make verify
```

`verify` は以下のターゲットを順に実行する:

```
localstack-up → ecr-push → ecs-deploy → kind-up → kind-deploy → compare
```

## ターゲット別 期待出力

### `make localstack-up`

```
[+] Running 2/2
 ✔ Container 03_ecs_vs_k8s-localstack-1  Started
 ✔ Container 03_ecs_vs_k8s-registry-1    Started
Waiting for LocalStack to be ready...
LocalStack ready.
```

### `make ecr-push`

```
...
ECR repo (local registry substitute): localhost:5000/demo-api
v1: digest: sha256:... size: 856
localhost:5000/demo-api
```

> **注**: LocalStack Community は ECR 非対応。`registry:2` (localhost:5000) を ECR 代替として使用。

### `make ecs-deploy`

```
--- terraform init ---
Terraform has been successfully initialized!
--- terraform apply (ECS resources will 501 on LocalStack Community) ---
aws_iam_role.task_exec: Creating...
aws_iam_role.task: Creating...
aws_cloudwatch_log_group.api: Creating...
aws_iam_role.task_exec: Creation complete ...
aws_iam_role.task: Creation complete ...
aws_cloudwatch_log_group.api: Creation complete ...
aws_ecs_cluster.this: Creating...
╷
│ Error: creating ECS Cluster (learning-ecs): ... StatusCode: 501
│ api error InternalFailure: API for service 'ecs' not yet implemented or pro feature
╵
...
WARN: terraform apply exited 1.
  LocalStack Community does not emulate ECS (Pro feature).
  Succeeded : aws_iam_role.task_exec, aws_iam_role.task, aws_cloudwatch_log_group.api
  Failed 501: aws_ecs_cluster.this, aws_ecs_task_definition.api, aws_ecs_service.api
  This is expected — continuing with k8s side.
```

> **注**: IAM / CloudWatch ログが作成されれば成功。make は exit 0 で次へ進む。

### `make kind-up`

```
Creating cluster "learning-ecs-vs-k8s" ...
 ✓ Ensuring node image (kindest/node:v1.35.0) 🖼
 ✓ Preparing nodes 📦
 ✓ Writing configuration 📜
 ✓ Starting control-plane 🕹️
 ✓ Installing CNI 🔌
 ✓ Installing StorageClass 💾
Set kubectl context to "kind-learning-ecs-vs-k8s"
```

クラスタが既に存在する場合は `|| true` でスキップ。

### `make kind-deploy`

```
Image: "demo-api:v1" with ID "sha256:..." not yet present on node "learning-ecs-vs-k8s-control-plane", loading...
...
deployment.apps/api created
service/api created
ingress.networking.k8s.io/api created
horizontalpodautoscaler.autoscaling/api created
configmap/api-config created
persistentvolumeclaim/api-data created
deployment.apps/api condition met
```

### `make compare`

```
============================================
=== ECS (LocalStack Community)
============================================
=== ECS task list (LocalStack Community) ===
WARN: ECS list-tasks returned an error (expected).
  LocalStack Community does not emulate ECS (Pro feature).
  ECS comparison is Terraform-declaration-level only.

============================================
=== k8s (kind: kind-learning-ecs-vs-k8s)
============================================
NAME                  READY   UP-TO-DATE   AVAILABLE   AGE
deployment.apps/api   2/2     2            2           ...
...
--- smoke test: /api/v1/echo ---
{"host":"api-xxx","runtime":"k8s","version":"v1"}
```

## 既知の制約 (LocalStack Community Edition)

| 機能 | 状況 | 代替手段 |
|---|---|---|
| ECR | **非対応** (Pro 機能) | `registry:2` を localhost:5000 で起動、`seed.sh` で push |
| ECS Cluster / Task / Service | **非対応** (Pro 機能) | Terraform 宣言まで。`terraform apply` は 501 で失敗するが Makefile が警告を出して続行 |
| ALB / NLB | **非対応** | Terraform 宣言のみ (ECS 同様) |
| ECS タスク実行 | **非対応** | k8s (kind) 側で同一 image を実際に起動して比較 |
| IAM, CloudWatch Logs, STS | **利用可** | LocalStack Community が正常にエミュレート |

ECS 側の「比較」は Terraform ファイル (`terraform/ecs/main.tf`) を読むことで達成する。  
k8s 側は `make kind-deploy` で実際にコンテナが起動し、`/api/v1/echo` へのリクエストが通る。

## ECS vs k8s 比較早見表

| 観点 | ECS (Fargate) | k8s (kind) |
|---|---|---|
| ワークロード定義 | Task Definition (JSON/HCL) | Deployment YAML |
| スケール | ECS Service desired_count + Auto Scaling | HPA (CPU 60%, 2-6 pod) |
| ネットワーク | awsvpc / ALB | Service ClusterIP + Ingress |
| ログ | CloudWatch Logs (`/ecs/demo-api`) | kubectl logs |
| 永続化 | EFS (宣言のみ) | PVC (kind 標準SC) |
| IAM / Secret | Task Execution Role | ServiceAccount / ConfigMap |

## 後始末

```sh
make down
```

以下を順に削除する:
1. `terraform destroy` (LocalStack の IAM / Logs リソース)
2. `docker compose down -v` (LocalStack + registry コンテナ + ボリューム)
3. `kind delete cluster` (kind クラスタ)

各ステップは失敗しても次へ進む (`-` prefix)。
