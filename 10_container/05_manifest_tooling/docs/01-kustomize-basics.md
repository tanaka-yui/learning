# 01 Kustomize basics

## base + overlay 概念

Kustomize は「素の YAML（base）」と「環境差分（overlay）」を分離し、継承によって環境ごとのマニフェストを生成するツールである。テンプレート言語を使わず、元の YAML を直接追記・上書きする点が特徴。

```
kustomize/
├── base/               # 共通リソース定義
│   ├── kustomization.yaml
│   ├── deployment.yaml
│   ├── service.yaml
│   └── configmap.yaml
├── components/         # 再利用可能なオプション単位
└── overlays/
    ├── dev/
    ├── stg/
    └── prod/
```

本実装では `kustomize/base/` に Deployment・Service・ConfigMap を配置し、各 overlay が `resources: [../../base]` で参照する（`kustomize/overlays/dev/kustomization.yaml` 参照）。

## 主要フィールド

| フィールド | 説明 | 実装例 |
|---|---|---|
| `resources` | 取り込む YAML / ディレクトリ | `resources: [../../base]` |
| `namespace` | 全リソースの namespace を上書き | `namespace: demo-dev` |
| `namePrefix` | 全リソース名に prefix を付与 | `namePrefix: dev-` |
| `nameSuffix` | 全リソース名に suffix を付与 | — |
| `commonLabels` | 全リソースに label を付与（注: Kustomize v5 以降は `labels` フィールドを推奨。`commonLabels` は `spec.selector` にも適用されるため、既存 Deployment への追加は再作成が必要） | `commonLabels: {chapter: "10-5"}` |
| `commonAnnotations` | 全リソースに annotation を付与 | — |

`kustomize/base/kustomization.yaml` では `commonLabels: {chapter: "10-5"}` を設定し、生成される全リソースに自動付与している。

## patches: strategic merge vs JSON6902

**Strategic Merge Patch** は Kubernetes のマージ戦略（`$patch: merge` / `$patch: delete`）に従い、リスト要素を key で識別してマージする。YAML 構造を元の manifest と同形式で記述する。

```yaml
# patch-replicas.yaml (kustomize/overlays/prod/patch-replicas.yaml)
apiVersion: apps/v1
kind: Deployment
metadata: { name: api }
spec: { replicas: 3 }
```

**JSON6902 Patch** は RFC 6902 に準拠した operation（add / remove / replace / move / copy / test）を配列で記述する。パスは JSON Pointer 形式を使い、配列末尾への追加（`-`）など精密な操作が可能。

```yaml
# patch-json6902.yaml (kustomize/overlays/prod/patch-json6902.yaml)
- op: add
  path: /spec/template/spec/containers/0/env/-
  value:
    name: PROD_FLAG
    value: "true"
```

prod overlay では両方を `patches:` に並列列挙し、strategic merge でレプリカ数を、JSON6902 で環境変数を個別に制御している。

## configMapGenerator / secretGenerator

`configMapGenerator` は ConfigMap を宣言的に生成し、デフォルトでは名前にコンテンツハッシュを付与する（immutable ConfigMap パターン）。Deployment がハッシュ付き名前を参照することで、設定変更時に Pod が自動ロールアウトされる。

`behavior` でマージ戦略を制御する。

| behavior | 動作 |
|---|---|
| `create`（デフォルト）| 新規 ConfigMap を作成 |
| `merge` | base の ConfigMap に値を追加・上書き |
| `replace` | base の ConfigMap 全体を置換 |

各 overlay では `behavior: merge` を指定し、base の `api-config` に `version=v1-dev` 等を追記している（`kustomize/overlays/dev/kustomization.yaml`）。

`secretGenerator` も同様に動作するが、`literals` の値は base64 エンコードされて Secret に格納される。本番環境ではリテラルを YAML に直書きせず、SealedSecrets / SOPS 等と組み合わせる。

## images フィールド

`images` フィールドを使うと、Deployment 内のコンテナイメージを patches なしで一括差替できる。

```yaml
images:
- name: demo-api          # マッチするイメージ名
  newTag: v2              # タグのみ差替
  # newName: registry/demo-api  # リポジトリも変更する場合
```

patches 版と比較して記述が簡潔だが、複数コンテナが混在する場合は `name` フィールドで正確に対象を絞り込む必要がある。CI からは `kustomize edit set image demo-api:$TAG` で動的に書き換えられる。
