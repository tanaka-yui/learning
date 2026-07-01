# 07 GHA Runner Controller (ARC)

実装参照: `apps/gha-runner/`

## アーキテクチャ

Actions Runner Controller (ARC) は GitHub Actions のセルフホストランナーを Kubernetes 上で管理する OSS。2023 年以降に新 API (AutoscalingRunnerSet) に刷新され、旧 API (RunnerDeployment) は非推奨となった。

```
GitHub Actions ジョブキュー
       ↓ webhook / polling
Controller (gha-runner-scale-set-controller)
       ↓ scale 指示
Listener Pod
       ↓ ランナー Pod 作成
Runner Pods (ジョブを 1 対 1 で実行)
```

- **Controller**: `arc-systems` Namespace に常駐。スケールセットを管理する。
- **Listener**: スケールセットごとに 1 Pod。GitHub からジョブ割り当てを待ち受ける。
- **Runner Pod**: ジョブ 1 件につき 1 Pod が起動し、完了後に削除される (Ephemeral)。

## AutoscalingRunnerSet (新 API) vs RunnerDeployment (旧 API)

| | AutoscalingRunnerSet | RunnerDeployment (非推奨) |
|---|---|---|
| API バージョン | `actions.github.com/v1alpha1` | `actions.summerwind.dev/v1alpha1` |
| スケール方式 | GitHub ジョブキューベース | Pod 数ベース |
| Ephemeral | デフォルト | オプション |
| containerMode | 対応 (kubernetes/dind) | 非対応 |
| 推奨度 | 現行推奨 | 移行前の参考用 |

本章の `apps/gha-runner/runnerdeployment.yaml` は移行前の参考として残してある。本番では `runnerscaleset.yaml` (AutoscalingRunnerSet) を使う。

## containerMode: kubernetes

```yaml
# apps/gha-runner/runnerscaleset.yaml (抜粋)
spec:
  containerMode: { type: kubernetes }
```

`containerMode: kubernetes` を指定すると、ランナー Pod 内で DinD (Docker in Docker) を使わずに、ワークフローの各ステップを別コンテナとして実行できる。DinD より安全でリソース効率も良い。

## GH PAT vs GitHub App

| | Personal Access Token (PAT) | GitHub App |
|---|---|---|
| スコープ | ユーザ権限に依存 | リポジトリ・組織単位で最小権限 |
| ローテーション | 手動 | 自動 (1 時間ごと) |
| 監査 | ユーザ名で追跡 | App 名で追跡 |
| 推奨 | 個人・PoC | 本番運用 |

認証情報は `gh-config` Secret に格納する:

```bash
kubectl create secret generic gh-config \
  --from-literal=github_token=<PAT_OR_APP_PRIVATE_KEY> \
  -n arc-runners
```

## オートスケール (Workflow Queue Based)

`minRunners: 0` / `maxRunners: 3` を設定すると、ジョブキューが空の時はランナーが 0 まで縮小し、ジョブ追加時に最大 3 Pod まで自動スケールする。コストを最小化できる点が CodeBuild との大きな違い。

## image.tag に関する注意点 (Task 7 finding)

`apps/gha-runner/install/values.yaml` では `image.tag` を意図的に指定していない:

```yaml
# image.tag は意図的に省略 — chart デフォルト (chart version に対応するタグ) を使う。
# tag: 0.9.3 を固定すると chart 0.14.2 の新フラグ (-runner-max-concurrent-reconciles) と
# 互換性がなくなり controller が起動しない。
image:
  repository: ghcr.io/actions/gha-runner-scale-set-controller
```

chart を upgrade する際はタグを明示的に固定せず、chart のデフォルトに任せること。

## CodeBuild との対応

| GHA Runner Controller | AWS CodeBuild |
|---|---|
| AutoscalingRunnerSet | CodeBuild Project (managed fleet) |
| containerMode: kubernetes | CodeBuild コンテナ環境 |
| `runs-on: self-hosted` | `environment.type: LINUX_CONTAINER` |
| minRunners: 0 / maxRunners | reserved capacity vs on-demand |
| GitHub App 認証 | CodeBuild Source (GitHub OAuth) |
| Kubernetes RBAC | IAM Role for CodeBuild |

**学習上の主な差**: CodeBuild はフルマネージドでインフラ管理不要。ARC は Kubernetes 上でランナーを完全制御でき、任意のコンテナイメージ・ストレージ・Kubernetes 連携が可能。

## 動作確認 (Makefile)

```bash
make arc-install     # controller + runnerscaleset を Helm でインストール
make arc-status      # kubectl -n arc-systems get pods
```

> **注意**: `AutoscalingRunnerSet` の apply には GitHub の実 PAT または GitHub App が必要。PAT なしの状態では controller のインストールは成功するが、RunnerSet の接続確立はできない。Makefile の `arc-install` は警告メッセージを表示する。
