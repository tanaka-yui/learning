# 10-4 GitOps + Argo エコシステム — 設計仕様

- 作成日: 2026-07-01
- 章: `10_container/04_gitops_argo/`
- 関連: `10-2` (demo-api 再利用、Istio と Rollouts 連携)、`10-3` (AWS 対比再利用)

## 目的

Argo エコシステム 4 本 (CD / Workflows / Rollouts / Events) と代替 (Flux / GHA Runner Controller) を「動く GitOps mono-repo + docs」で学習。Kube が CD 用の tool を自前で選ぶ必要がある点を、AWS マネージド (CodePipeline / Step Functions / EventBridge / CodeDeploy) との対比で明確化。

## スコープ

- **Argo CD**: Application / ApplicationSet / AppProject / RBAC / self-heal / sync wave
- **Flux**: GitRepository / Kustomization / HelmRelease / Notification (Argo CD 対比)
- **Argo Workflows**: DAG + step template、WorkflowTemplate、artifact、CronWorkflow
- **Argo Rollouts**: Canary + Analysis (Prometheus)、Istio と連携
- **Argo Events**: EventBus + EventSource (webhook) + Sensor → Workflow trigger
- **GHA Runner Controller (ARC)**: k8s 上に self-hosted GHA runner
- **AWS 対比**: CodePipeline / CodeBuild / CodeDeploy / Step Functions / EventBridge の Terraform stub + docs 比較表

スコープ外:
- Tekton (Argo Workflows 対比の別選択肢) — 別章余地
- Flagger (Argo Rollouts 対比の別選択肢) — docs 内 1 段落のみ言及
- Argo CD Notifications 詳細 (基本のみ)
- 実 AWS デプロイ (LocalStack で概念確認)

## アーキテクチャ

```
10_container/04_gitops_argo/
├── docs/
│   ├── README.md
│   ├── 01-gitops-basics.md
│   ├── 02-argocd.md
│   ├── 03-flux.md
│   ├── 04-argo-workflows.md
│   ├── 05-argo-rollouts.md
│   ├── 06-argo-events.md
│   ├── 07-gha-runner-controller.md
│   ├── 08-aws-comparison.md
│   └── decision-matrix.md
├── base/
│   ├── deployment.yaml
│   ├── service.yaml
│   ├── analysistemplate.yaml
│   └── kustomization.yaml
├── envs/
│   ├── dev/{kustomization.yaml, patch-image.yaml}   # replicas=1, image tag=v1
│   ├── stg/{kustomization.yaml, patch-image.yaml}   # replicas=2, image tag=v1
│   └── prod/{kustomization.yaml, patch-image.yaml}  # replicas=3, image tag=v1 (Rollout 適用対象)
├── apps/
│   ├── argocd/
│   │   ├── install/values.yaml
│   │   ├── projects/demo.yaml
│   │   ├── applications/{demo-dev.yaml, demo-stg.yaml, demo-prod.yaml}
│   │   └── applicationset/demo.yaml
│   ├── flux/
│   │   ├── install/README.md
│   │   ├── gitrepository.yaml
│   │   ├── kustomization-dev.yaml
│   │   └── notification.yaml
│   ├── workflows/
│   │   ├── install/values.yaml
│   │   ├── workflowtemplate-shared.yaml
│   │   ├── ci-build-promote.yaml
│   │   └── cron-report.yaml
│   ├── rollouts/
│   │   ├── install/README.md
│   │   ├── rollout-canary.yaml
│   │   └── analysistemplate.yaml
│   ├── events/
│   │   ├── install/README.md
│   │   ├── eventbus.yaml
│   │   ├── eventsource-webhook.yaml
│   │   └── sensor-trigger-workflow.yaml
│   └── gha-runner/
│       ├── install/values.yaml
│       ├── runnerdeployment.yaml
│       └── runnerscaleset.yaml
├── comparison/
│   └── aws/
│       ├── terraform/
│       │   ├── providers.tf
│       │   ├── variables.tf
│       │   ├── codepipeline.tf
│       │   ├── eventbridge.tf
│       │   ├── stepfunctions.tf
│       │   └── outputs.tf
│       └── README.md
├── Makefile
├── README.md
└── VERIFICATION.md
```

## base/ と envs/ のパッチ例

`base/deployment.yaml` は 10-2 の demo-api を GitOps 用にリパッケージ:
- image: `localhost:5001/demo-api:v1` (10-2 で registry に push 済)
- replicas: 1 (base のデフォルト、overlays で上書き)
- env `APP_VERSION` / `RUNTIME` は ConfigMap `api-config` から

`envs/dev/kustomization.yaml`:
```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: demo-dev
resources: [../../base]
patches:
- path: patch-image.yaml
  target: { kind: Deployment, name: api }
```

`envs/dev/patch-image.yaml` (strategic merge):
```yaml
apiVersion: apps/v1
kind: Deployment
metadata: { name: api }
spec:
  replicas: 1
  template:
    spec:
      containers:
      - name: api
        image: localhost:5001/demo-api:v1
        env:
        - { name: APP_VERSION, value: v1-dev }
```

`envs/prod/patch-image.yaml`: 同形式で `replicas: 3`, `APP_VERSION: v1-prod`。GitOps flow で「この 1 ファイルの image tag を書き換えて commit する」だけで sync が起きる、が中核学習体験。

## 各コンポーネント要点

### Argo CD (`apps/argocd/`)

- Helm install: `argo-cd` chart (バージョン 7.x)、kind 用に `server.service.type=NodePort`、`configs.params."server.insecure"=true`
- `projects/demo.yaml`: AppProject `demo`、`sourceRepos: [<this repo url>]`、`destinations: [{namespace: 'demo-*', server: 'https://kubernetes.default.svc'}]`
- `applications/demo-{dev,stg,prod}.yaml`: 各環境ごとの Application、`spec.source.path=envs/<env>`、`spec.syncPolicy.automated={selfHeal: true, prune: true}`
- `applicationset/demo.yaml`: List Generator で 3 環境を宣言、Application を自動生成 (推奨形)

### Flux (`apps/flux/`)

- `flux install` で control plane 導入 (bootstrap は使わず、mono-repo pattern を明示)
- `gitrepository.yaml`: `GitRepository/demo` を repo URL + branch 参照
- `kustomization-dev.yaml`: `Kustomization` で `envs/dev` を watch、`interval: 1m`
- `notification.yaml`: `Alert` + `Provider` (kind では slack は使えないので、`generic` = webhook echo)

### Argo Workflows (`apps/workflows/`)

- Helm install: `argo-workflows` chart
- `workflowtemplate-shared.yaml`: `WorkflowTemplate` に共通の `templates: [buildkit, git-commit]` を宣言
- `ci-build-promote.yaml`: `Workflow` DAG:
  1. `git-clone` → 2. `buildkit-build` (image build using BuildKit) → 3. `trivy-scan` → 4. `git-commit` (envs/dev/patch-image.yaml の tag 更新)
  - artifact passing (image digest を tag として commit)
- `cron-report.yaml`: `CronWorkflow` 1h ごとに `argocd app diff` 相当の drift 検出

### Argo Rollouts (`apps/rollouts/`)

- Rollouts controller install (`kubectl apply -f https://github.com/argoproj/argo-rollouts/releases/download/v1.7.2/install.yaml`)
- `rollout-canary.yaml`: prod ns で Deployment を Rollout に置換:
  ```yaml
  apiVersion: argoproj.io/v1alpha1
  kind: Rollout
  metadata: { name: api, namespace: demo-prod }
  spec:
    replicas: 3
    selector: { matchLabels: { app: api } }
    template:
      metadata: { labels: { app: api } }
      spec:
        containers:
        - name: api
          image: localhost:5001/demo-api:v1
          ports: [{ containerPort: 8080 }]
    strategy:
      canary:
        steps:
        - setWeight: 20
        - pause: { duration: 30s }
        - setWeight: 50
        - analysis:
            templates: [{ templateName: pod-ready }]
        - setWeight: 100
  ```
- `analysistemplate.yaml`: Prometheus クエリで success rate > 95% を判定 (kind 上には Prometheus 無いので、`analysis: kubectl` フォールバックで pod ready 数を counter)。docs で Prometheus 統合の本番形も並記。

### Argo Events (`apps/events/`)

- Argo Events install (`kubectl apply -f https://raw.githubusercontent.com/argoproj/argo-events/stable/manifests/install.yaml` + validating webhook)
- `eventbus.yaml`: `EventBus/default` (JetStream backend)
- `eventsource-webhook.yaml`: `EventSource/webhook` で HTTP endpoint を expose
- `sensor-trigger-workflow.yaml`: `Sensor` が webhook event を受けて `Workflow` を submit

### GHA Runner Controller (`apps/gha-runner/`)

- Actions Runner Controller (ARC) chart install
- `runnerscaleset.yaml`: `AutoscalingRunnerSet` で 0-3 replicas、GH org を label で選択
- `runnerdeployment.yaml`: (旧 API) 参考として並置
- **注**: 本物 GH repo との連携は PAT / GitHub App 認証必要。kind 上では controller install + CRD 適用まで、実 job pull は docs 内で外部 GH repo との連携手順を記載

### AWS 対比 (`comparison/aws/`)

Terraform stub (10-3 の Terraform + LocalStack 環境を再利用):

- `codepipeline.tf`: CodePipeline + CodeBuild + CodeDeploy (LocalStack Community で ECS が動かないのは 10-3 で判明済 → 宣言のみ)
- `eventbridge.tf`: EventBridge Rule + Target (Step Functions)
- `stepfunctions.tf`: State Machine ("choice→parallel→map" 構成)、LocalStack で `describe-state-machine` 確認可能な範囲

docs/08-aws-comparison.md 対応表:

| 概念 | Kube (Argo) | AWS |
|---|---|---|
| GitOps CD | Argo CD / Flux | CodePipeline + CodeDeploy (git source) |
| Pipeline orchestration | Argo Workflows | Step Functions / CodeBuild |
| Progressive delivery | Argo Rollouts | CodeDeploy Blue/Green |
| Event trigger | Argo Events | EventBridge + SNS/SQS |
| Runner / worker | GHA Runner Controller | CodeBuild / GitHub Actions Runner (SaaS) |
| Container registry | External (ECR/GHCR/Docker Hub) | ECR |

## GitOps flow

```
1. envs/dev/patch-image.yaml で image tag `demo-api:v1` → `v1.1` に書換
2. git commit + push
3. Argo CD polls (default 3m; --sync 手動で即時)
4. Application status=OutOfSync → automated sync → dev 環境更新
5. dev で smoke OK
6. PR で envs/stg/patch-image.yaml も更新、merge
7. stg 更新確認後、envs/prod で Rollout canary が起動、Analysis 通過で 100%
```

## Makefile 骨子

```makefile
.PHONY: up install-argocd install-flux install-workflows install-rollouts \
        install-events install-gha demo-sync demo-canary demo-workflow \
        demo-event install-aws-comparison verify down

up:
	# learning-base cluster 前提 (10-2 の kind cluster.yaml を流用)
	kind create cluster --config ../02_kind_mesh/kind/cluster.yaml || true

install-argocd:
	helm upgrade --install argocd argo-cd \
	  --repo https://argoproj.github.io/argo-helm --version 7.7.0 \
	  -n argocd --create-namespace -f apps/argocd/install/values.yaml
	kubectl -n argocd wait --for=condition=Available deploy --all --timeout=300s
	kubectl apply -f apps/argocd/projects/demo.yaml
	kubectl apply -f apps/argocd/applicationset/demo.yaml

install-flux:
	flux install
	kubectl apply -f apps/flux/gitrepository.yaml
	kubectl apply -f apps/flux/kustomization-dev.yaml
	kubectl apply -f apps/flux/notification.yaml

install-workflows:
	helm upgrade --install argo-workflows argo-workflows \
	  --repo https://argoproj.github.io/argo-helm \
	  -n argo --create-namespace -f apps/workflows/install/values.yaml
	kubectl apply -f apps/workflows/workflowtemplate-shared.yaml

install-rollouts:
	kubectl create namespace argo-rollouts || true
	kubectl apply -n argo-rollouts \
	  -f https://github.com/argoproj/argo-rollouts/releases/download/v1.7.2/install.yaml

install-events:
	kubectl apply -f https://raw.githubusercontent.com/argoproj/argo-events/v1.9.2/manifests/install.yaml
	kubectl apply -f apps/events/eventbus.yaml
	kubectl apply -f apps/events/eventsource-webhook.yaml
	kubectl apply -f apps/events/sensor-trigger-workflow.yaml

install-gha:
	helm upgrade --install arc oci://ghcr.io/actions/actions-runner-controller-charts/gha-runner-scale-set-controller \
	  -n arc-systems --create-namespace
	# runnerscaleset は GH PAT が必要なため kind では apply skip (docs で手順)

demo-sync:
	@# envs/dev の image tag を書き換え → git commit → argocd sync 観察
	@echo "hand-run: edit envs/dev/patch-image.yaml image tag, git commit, then argocd app sync demo-dev"

demo-canary:
	kubectl argo rollouts get rollout api -n demo-prod --watch

demo-workflow:
	argo submit --serviceaccount argo -n argo apps/workflows/ci-build-promote.yaml --watch

demo-event:
	curl -X POST http://localhost:12000/example -d '{"trigger":"go"}'
	argo list -n argo | head

install-aws-comparison:
	docker compose -f ../03_ecs_vs_k8s/localstack/docker-compose.yml up -d
	cd comparison/aws/terraform && terraform init && terraform apply -auto-approve

verify:
	$(MAKE) install-argocd
	$(MAKE) install-workflows
	$(MAKE) install-rollouts
	$(MAKE) install-events
	$(MAKE) demo-workflow
	$(MAKE) demo-event

down:
	kubectl delete -f apps/events/sensor-trigger-workflow.yaml --ignore-not-found
	kubectl delete -f apps/events/eventbus.yaml --ignore-not-found
	kubectl delete -f apps/argocd/applicationset/demo.yaml --ignore-not-found
	helm uninstall argocd -n argocd || true
	helm uninstall argo-workflows -n argo || true
	helm uninstall arc -n arc-systems || true
	flux uninstall --silent || true
```

## docs/ 構成

各 docs は 300-600 words、cross-link あり、`docs/08-aws-comparison.md` と `docs/decision-matrix.md` を最後に読ませる構成。

## 検証

- `make verify`: 4 Argo tool install + demo-workflow + demo-event 成功
- `make demo-canary`: Rollout の Weight 遷移が観察可能
- `make demo-sync`: envs/dev 変更 → Application が Synced 遷移 (手動 sync でも可)
- VERIFICATION.md: 段階的 install 手順、kind リソース制約 (10-2 の base cluster 過負荷経験を踏まえ、install-argocd と install-workflows は同時ではなく順次)、AWS 対比 LocalStack 限界注記

## テスト

- `kubeconform -strict` で全 manifest 静的検証
- Argo CD Application が Synced に到達するまでを polling
- Workflow が Succeeded status 到達
- Rollout の Analysis run が Successful

## 既存資産との関係

- demo-api image (`demo-api:v1` / `v2`): 10-2 で push 済 `localhost:5001/demo-api:v1|v2` を再利用
- Istio (Rollouts と mesh 連携): 10-2 の istio-sidecar overlay 導入前提 (VERIFICATION で明示)
- kind cluster: 10-2 の `learning-base` を再利用
- LocalStack: 10-3 の docker-compose を再利用
- Terraform pattern: 10-3 の providers.tf を base に endpoint override
