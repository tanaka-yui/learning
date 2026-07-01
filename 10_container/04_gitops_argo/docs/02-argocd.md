# 02 Argo CD

実装参照: `apps/argocd/`

## 主要リソース

### Application

Argo CD の最小デプロイ単位。「どの Git リポジトリのどのパスを」「どのクラスタのどの Namespace に」デプロイするかを宣言する。

```yaml
spec:
  source:
    repoURL: https://github.com/tanaka-yui/learning.git
    targetRevision: HEAD
    path: 10_container/04_gitops_argo/envs/dev
  destination:
    server: https://kubernetes.default.svc
    namespace: demo-dev
  syncPolicy:
    automated: { prune: true, selfHeal: true }
```

`apps/argocd/applications/` 以下に `demo-dev/stg/prod` の 3 Application を定義している。

### ApplicationSet

Application をテンプレートで量産するリソース。ジェネレータ (List/Git/Matrix 他) が変数を生成し、template に展開する。本章では List ジェネレータで env ごとの Application を自動生成している (`apps/argocd/applicationset/demo.yaml`)。

```yaml
generators:
- list:
    elements:
    - { env: dev,  replicas: "1" }
    - { env: stg,  replicas: "2" }
    - { env: prod, replicas: "3" }
template:
  metadata: { name: 'demo-{{env}}' }
  spec:
    source:
      path: '10_container/04_gitops_argo/envs/{{env}}'
```

### AppProject

Application をグループ化しアクセス制御する。`sourceRepos`・`destinations`・`clusterResourceWhitelist`・`namespaceResourceWhitelist` で操作範囲を絞れる。本章の `demo` プロジェクト (`apps/argocd/projects/demo.yaml`) は `demo-*` Namespace に制限している。

### RBAC

Argo CD 独自の RBAC は `argocd-rbac-cm` ConfigMap に記述する。`p, role:dev-team, applications, sync, demo/*, allow` のようにプロジェクト・アクション・リソースを指定。Argo CD のロール (`role:admin`, `role:readonly`) をベースに拡張できる。

### SSO (Dex)

Dex は OIDC ブローカーとして GitHub / Google / LDAP などの IdP と Argo CD を橋渡しする。本章の `install/values.yaml` では `dex.enabled: false` (kind 環境での簡略化)。本番では Dex を有効化し GitHub OAuth App を設定する。

### Sync Policy

| フィールド | 意味 |
|---|---|
| `automated: {}` | Git 変更を検知して自動 sync |
| `prune: true` | Git から消えたリソースを削除 |
| `selfHeal: true` | クラスタの手動変更を Git に戻す |
| `syncOptions: [CreateNamespace=true]` | Namespace が無ければ自動作成 |

自動 sync を無効にして手動承認フローにすることも可能 (`syncPolicy` を省略)。

### Sync Wave

リソースへのアノテーション `argocd.argoproj.io/sync-wave: "N"` で順序制御する。詳細は `01-gitops-basics.md` 参照。

### Hooks (PreSync / PostSync / SyncFail)

sync ライフサイクルに Job/Pod を差し込む機能。`argocd.argoproj.io/hook: PreSync` を付けた Job は sync 前に実行され、完了後に本体 sync が走る。DB マイグレーション・スモークテストなどに使う。`argocd.argoproj.io/hook-delete-policy: HookSucceeded` で成功後に自動削除できる。

### Notifications

`argocd-notifications-cm` ConfigMap でトリガー・テンプレートを定義し、Slack / PagerDuty / メール などに通知を送れる。本章では `notifications.enabled: false` (kind 環境での簡略化)。

## 運用コマンド

```bash
# Application 一覧
argocd app list

# 手動 sync
argocd app sync demo-dev

# diff 確認
argocd app diff demo-dev

# ロールバック (直前の履歴に戻す)
argocd app rollback demo-dev
```

## kind 環境での注意点

`install/values.yaml` で `server.insecure: "true"` と `service.type: NodePort` を設定している。本番では TLS 終端 (Ingress + cert-manager) と LoadBalancer を使う。
