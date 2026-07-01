# 03 Flux

実装参照: `apps/flux/`

## 主要リソース

### GitRepository

Git リポジトリの参照と fetch 間隔を定義する Source リソース。

```yaml
# apps/flux/gitrepository.yaml
apiVersion: source.toolkit.fluxcd.io/v1
kind: GitRepository
metadata: { name: demo, namespace: flux-system }
spec:
  interval: 1m
  url: https://github.com/tanaka-yui/learning.git
  ref: { branch: main }
```

SSH キーや GitHub App を `secretRef` で指定してプライベートリポジトリにも対応できる。

### OCIRepository

OCI レジストリに push した Kustomization バンドルや Helm チャートを Source として参照する。`flux push artifact` でレジストリに publish し、Git ではなくコンテナイメージとしてマニフェストを配布できる。

### Kustomization

Source を参照してクラスタに適用する Reconciler。`path` に kustomize ディレクトリを指定する。

```yaml
# apps/flux/kustomization-dev.yaml
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata: { name: demo-dev, namespace: flux-system }
spec:
  interval: 2m
  path: ./10_container/04_gitops_argo/envs/dev
  prune: true
  sourceRef: { kind: GitRepository, name: demo }
  targetNamespace: demo-dev-flux
```

`prune: true` で Git から消えたリソースを自動削除する。`dependsOn` で他の Kustomization が Ready になってから適用する順序制御も可能。

### HelmRelease

Helm チャートの install/upgrade を宣言的に管理する。`chartRef` (OCIRepository) または `chart.spec.sourceRef` (HelmRepository) を指定し、`values` でオーバーライドを記述する。

### Alert & Provider (Notification)

```yaml
# apps/flux/notification.yaml
kind: Provider      # 通知先 (generic webhook / Slack / Teams / PagerDuty 等)
kind: Alert         # どの Source/Kustomization のどの severity を通知するか
```

本章では echo-server への generic webhook Provider と `demo-dev` Kustomization の `info` Alert を定義している。

### Image Automation (ImageRepository / ImagePolicy)

コンテナレジストリのタグを監視し、ポリシー (semver/regex) に合うタグに自動更新する機能。`ImageRepository` でレジストリを指定し、`ImagePolicy` で選択ポリシーを定義。`ImageUpdateAutomation` が Git にコミットを自動 push する。マニフェスト中に `# {"$imagepolicy": "flux-system:my-policy"}` コメントを書いてタグ挿入箇所を指定する。

## Argo CD との比較

| 観点 | Argo CD | Flux |
|---|---|---|
| sync 方式 | pull (+ push モード optional) | pull only |
| UI | 標準搭載 (Web UI / CLI) | 標準 UI なし (flux CLI / Weave GitOps OSS) |
| App-of-Apps vs chain | App-of-Apps / ApplicationSet | Kustomization の `dependsOn` でチェーン |
| Multi-tenancy | AppProject + RBAC | Tenant ごとに namespace 分離 + ServiceAccount |
| Image 自動更新 | ArgoCD Image Updater (別途インストール) | Image Automation (標準機能) |
| 通知 | argocd-notifications | Notification Controller (標準機能) |
| 学習コスト | UI があり直感的 | CRD の組み合わせを理解する必要あり |

## アンチパターン

同一クラスタに Argo CD と Flux の両方を導入し、同一 Namespace の同一リソースを管理しようとすると reconcile の競合が発生する。本章では Argo CD が `demo-dev/stg/prod` を管理し、Flux が `demo-dev-flux` を管理することで分離している。

## 運用コマンド

```bash
# Kustomization の同期状態確認
flux get kustomizations

# 強制リコンサイル
flux reconcile kustomization demo-dev --with-source

# ログ確認
flux logs --kind=Kustomization --name=demo-dev
```
