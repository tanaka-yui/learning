# 10-4 検証手順

## 前提条件

| ツール | バージョン目安 |
|--------|--------------|
| kind | v0.24+ |
| kubectl | v1.30+ |
| helm | v3.14+ |
| flux CLI | v2.3+ |
| argocd CLI | v2.11+ |
| kubectl-argo-rollouts プラグイン | v1.7+ |
| terraform | v1.8+ |
| docker / docker compose | v26+ / v2.27+ |
| jq | 任意 |

- kind cluster `learning-base` (10-2 の `cluster.yaml` を使用)
- ローカルレジストリ `localhost:5001` が稼働中
- 10-2 で `demo-api:v1` / `:v2` が registry に push 済

---

## インストール手順

```sh
cd 10_container/04_gitops_argo

# 1. cluster 起動 (既存なら skip)
make up

# 2. Argo CD
make install-argocd

# 3. Flux (オプション — Argo CD と ns 分離済)
make install-flux

# 4. Argo Workflows
make install-workflows

# 5. Argo Rollouts
make install-rollouts

# 6. Argo Events
make install-events

# 7. ARC controller (controller のみ / PAT 不要)
make install-gha

# 8. AWS 比較 (LocalStack + Terraform)
make install-aws-comparison

# 一括検証 (up + argocd + workflows + rollouts + events + demo-workflow + demo-event)
make verify
```

---

## デモ

```sh
# Workflow を手動サブミット
make demo-workflow

# Webhook イベントで Workflow をトリガー
make demo-event

# Rollout カナリアデモ (demo-api:v2 に更新)
make demo-canary

# Argo CD GitOps 同期 (手動手順を表示)
make demo-sync
```

---

## 期待される結果

| 検証項目 | コマンド | 期待値 |
|----------|---------|--------|
| Argo CD Application | `argocd app list` | demo-dev / demo-stg / demo-prod が Synced または OutOfSync |
| Workflows デプロイ | `kubectl -n argo get deploy` | argo-workflows-server, argo-workflows-workflow-controller が Available |
| Workflow 実行 | `kubectl -n argo get wf` | ci-build-promote-\<hash\> が Succeeded |
| Event トリガー | `make demo-event` 後に `kubectl -n argo get wf` | event-triggered-\<hash\> が作成・Succeeded |
| Rollout カナリア | `kubectl argo rollouts -n demo-prod get rollout api` | Weight 20 → 50 → analysis → 100 |
| Terraform | `terraform -chdir=comparison/aws/terraform show` | state machine + eventbridge rule が apply 成功 |

---

## 既知の制限・ハマりポイント

### Argo Workflows: CRD install Job 失敗

クラスター負荷が高い場合、Helm の CRD install Job がタイムアウトすることがある。

**回避策:**
```sh
# CRD を手動 apply
kubectl apply -k \
  https://github.com/argoproj/argo-workflows/manifests/base/crds/full?ref=v3.5.10

# --set crds.install=false で再インストール
helm upgrade --install argo-workflows argo/argo-workflows \
  -n argo --create-namespace \
  -f apps/workflows/install/values.yaml \
  --set crds.install=false
```

### Argo Events: operate-workflow-sa + RoleBinding

`install-events` ターゲットは以下を自動作成する:
- ServiceAccount `operate-workflow-sa` (namespace: `argo-events`)
- Role `workflow-creator` (namespace: `argo`)
- RoleBinding `operate-workflow-sa-binding` (namespace: `argo`, subject: `argo-events/operate-workflow-sa`)

手動での確認・再作成:
```sh
kubectl get sa operate-workflow-sa -n argo-events
kubectl get rolebinding operate-workflow-sa-binding -n argo
```

### Argo Rollouts: Readiness に時間がかかる

クラスター負荷が高い場合、Rollout の Ready 状態への移行に 5 分以上かかることがある。`make install-rollouts` は `--timeout=300s` を設定しているが、タイムアウトした場合は以下で状態を確認:

```sh
kubectl -n argo-rollouts get deploy
kubectl argo rollouts -n demo-prod get rollout api
```

### ARC (GitHub Actions Runner Controller): PAT / GitHub App 必須

`make install-gha` は controller のみをインストールする。実際の Runner を登録するには GitHub PAT または GitHub App が必要。

`AutoscalingRunnerSet` の適用には GitHub PAT を設定した後:
```sh
kubectl create secret generic gh-pat \
  -n arc-systems \
  --from-literal=github_token=<PAT>
kubectl apply -f apps/gha-runner/runnerscaleset.yaml
```

詳細: `docs/07-gha-runner-controller.md`

### LocalStack: events / stepfunctions が未起動

`make install-aws-comparison` は 10-3 の `docker-compose.yml` をそのまま使う。  
そのファイルの `SERVICES` は `iam,logs,cloudwatch,sts` のみで、`events` と `stepfunctions` が含まれていない。

**回避策 (どちらか):**

A) `10_container/03_ecs_vs_k8s/localstack/docker-compose.yml` の `SERVICES` を一時的に編集:
```yaml
SERVICES: iam,logs,cloudwatch,sts,events,stepfunctions
```

B) LocalStack コンテナを再起動して追加サービスを有効化:
```sh
docker restart localstack
```

### CodePipeline: LocalStack Community 非対応 (501)

`comparison/aws/terraform/codepipeline.tf` は宣言のみ。LocalStack Community は CodePipeline を未実装のため `terraform apply` は 501 エラーになる。宣言の存在確認のみが目的。

### Argo CD Application が Missing になる場合

`spec.source.repoURL` がプライベートリポジトリまたは未 push の場合に発生。  
公開リポジトリに push するか、ApplicationSet の repoURL を実際の fork に変更:
```sh
argocd repo add https://github.com/<your-fork>/learning --username <user> --password <token>
argocd app sync demo-dev
```

---

## Teardown

```sh
make down
```

`down` ターゲットは以下を削除する:
- Argo Events リソース (sensor, eventbus)
- Rollout リソース
- Helm releases: argocd, argo-workflows, arc
- Flux
- 関連 namespace: argocd / argo / argo-events / argo-rollouts / flux-system / arc-systems / demo-*
