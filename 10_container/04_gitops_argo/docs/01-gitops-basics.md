# 01 GitOps 基礎

## GitOps 4 原則

GitOps は Weaveworks が提唱した CD (継続的デリバリー) の運用パターンで、以下 4 つの原則に基づく。

1. **宣言的 (Declarative)**: インフラとアプリのあるべき状態を YAML/JSON で宣言する。手順書ではなく「状態」を記述することで冪等性を担保する。
2. **バージョン管理 (Versioned & Immutable)**: 宣言したマニフェストを Git に保存し、変更履歴・ロールバックを Git のコミット履歴で管理する。
3. **自動適用 (Pulled Automatically)**: Git の変更を起点に、クラスタ側エージェント (Argo CD / Flux) が自動的に pull して適用する。外部からクラスタへの push アクセスは不要になる。
4. **継続的調整 (Continuously Reconciled)**: エージェントが定期的に Git の状態とクラスタの実態を比較し、乖離があれば自動修正 (reconcile) する。

## Push 型 vs Pull 型 CD

| | Push 型 (従来 CI/CD) | Pull 型 (GitOps) |
|---|---|---|
| トリガー | CI が `kubectl apply` を実行 | クラスタ内エージェントが Git を監視 |
| クレデンシャル | CI に kubeconfig が必要 | クラスタ内に秘密情報を封じ込め |
| ドリフト検出 | なし (手動確認) | 継続的に検出・自動修正可能 |
| 監査ログ | CI ログ + クラスタ audit | Git コミット履歴 |

Push 型は実装が単純だが、CI サーバがクラスタの書き込み権限を持つため攻撃面が広がる。Pull 型はクラスタ外への書き込み権限を不要とし、セキュリティ境界が明確になる。

## ドリフト検出と自動修正

「ドリフト (Drift)」とは、Git の宣言状態とクラスタの実態が乖離した状態を指す。原因例:

- 誰かが手動で `kubectl edit` した
- ノード障害後に Pod 数が変化した
- Namespace を直接削除した

Argo CD は 3 分ごと (デフォルト)、Flux は `.spec.interval` 指定 (本章では 2 分) でリコンサイルを実行する。`selfHeal: true` を設定すると Git の状態に自動復元する。`prune: true` を設定すると Git から削除されたリソースをクラスタからも削除する。

## Sync Wave

複数リソースを一度にデプロイする場合、依存関係を考慮した順序制御が必要になる。Argo CD は `argocd.argoproj.io/sync-wave: "N"` アノテーションで順序を指定する。数値が小さいほど先にデプロイされ、各 Wave のリソースが Healthy になってから次の Wave が開始する。

典型的な使い方:
- `sync-wave: "0"` → Namespace / CRD を先に作成
- `sync-wave: "1"` → ConfigMap / Secret
- `sync-wave: "2"` → Deployment / Service

## App-of-Apps パターン

Argo CD の Application を管理する親 Application を用意するパターン。ルートリポジトリに各チームの Application マニフェストを置き、ルート Application がそれを同期することで全体を Git 一本で管理できる。本章では ApplicationSet の List ジェネレータで `dev/stg/prod` の 3 Application を自動生成している (`apps/argocd/applicationset/demo.yaml`)。

ApplicationSet はテンプレートエンジンとして `{{ env }}` のような変数を展開するため、環境追加時は generators の elements を 1 行追加するだけでよい。

## Mono-Repo vs 2-Repo 構成

| | Mono-Repo | 2-Repo (app repo + config repo) |
|---|---|---|
| 管理 | シンプル | 役割分担が明確 |
| アクセス制御 | 難しい (全員が同一 repo) | CI は app repo のみ、CD は config repo |
| スケール | 大規模では PR 衝突が増える | チーム別 config repo に分割可 |
| 監査 | マニフェストと実装が同居 | config repo が承認履歴の正となる |

本章はシングルリポジトリ構成で学習コストを下げている。本番では config repo を分離し、CI パイプラインが image tag だけを config repo に書き込む形が一般的。

## Kubernetes CD の選択肢 vs マネージド (CodePipeline)

Kubernetes には組み込みの CD 機能がない。そのため Argo CD / Flux などの OSS を自前でインストールして構成する必要がある。この「自前で選ぶ」性質が Kubernetes の強みであり、ツールチェーンの柔軟性をもたらす。

一方 AWS CodePipeline は完全マネージドで運用負荷は低いが、Kubernetes 固有の概念 (Rollout, SyncWave, RBAC, multi-tenancy) とは切り離された存在で、高度な Kubernetes 運用には別途 CD ツールが必要になる。

**学習指針**: まず Kubernetes ネイティブの GitOps ツール (Argo CD) で 4 原則を体験してから、AWS マネージドサービスとのトレードオフを理解するのが効率的。
