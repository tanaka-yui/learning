# compare: Kustomize vs Helm

## 選択マトリクス

| 観点 | Kustomize | Helm |
|---|---|---|
| 学習曲線 | 低（YAML 追記のみ） | 中〜高（Go template + ライフサイクル） |
| 表現力 | 中（patch による差分） | 高（条件分岐・ループ・関数） |
| 型安全 | 弱（YAML マージのみ） | `values.schema.json` で JSON Schema 検証可 |
| OSS 配布 | 弱（ディレクトリ共有のみ） | 強（Chart repository / OCI registry） |
| dry-run diff | `kubectl diff -k` / `kustomize build \| kubectl diff` | `helm diff` plugin / `helm template \| kubectl diff` |
| CRD 管理 | `resources:` に CRD を列挙し apply 順序を手動制御 | Chart.yaml の `crds/` dir で CRD を先行 apply |
| release 管理 | なし（kubectl の状態のみ） | `helm list` / `helm history` / `helm rollback` |
| フック | なし（apply 順序のみ） | pre/post install/upgrade/delete/test |

**Kustomize が向く場面:**

- クラスター内部リソースの差分管理のみ（外部配布なし）
- 既存 YAML をそのまま維持したい（テンプレート化したくない）
- 学習コストを最小化したい小規模チーム

**Helm が向く場面:**

- 外部チームへ Chart として配布する
- 値の型検証やリリース履歴・rollback が必要
- pre/post hook で DB マイグレーション等を自動化したい

## hybrid pattern

### Kustomize `helmCharts:` フィールド

Kustomize v4.1 以降、`kustomization.yaml` に `helmCharts:` を記述すると Kustomize が内部で `helm template` を呼び出し、生成された YAML を通常の resource として扱える。Helm の値制御と Kustomize の patch を組み合わせられる。

```yaml
helmCharts:
- name: demo-api
  releaseName: demo
  version: 0.1.0
  repo: oci://registry.example/charts
  valuesFile: helm/values-prod.yaml
```

生成結果に追加の `patches:` を重ねることで、Chart を変更せずにクラスター固有の設定を注入できる。

### Helm `--post-renderer`

`helm install --post-renderer` に実行可能スクリプトを渡すと、Helm が生成した YAML を stdin から受け取り加工した YAML を stdout に返す。kustomize を post-renderer として使うことで、Chart のテンプレート結果に Kustomize patch を重ねられる。

```bash
helm install demo helm/charts/demo-api \
  --post-renderer ./kustomize-post-render.sh
```

```bash
# kustomize-post-render.sh の例
cat > /tmp/helm-output.yaml
kustomize build /tmp/post-render-overlay
```

post-renderer は Helm 3.1 以降で利用可能。サードパーティ Chart を変更せずに追加リソースを注入したい場合に有効。

## Helmfile

Helmfile は複数 Helm release を宣言的に管理するオーケストレーションツール。`helmfile.yaml` に全 release を記述し、`helmfile apply` で一括 sync する。

```yaml
releases:
- name: demo-dev
  namespace: demo-dev-helm
  chart: ./helm/charts/demo-api
  values: [helm/values-dev.yaml]

- name: demo-prod
  namespace: demo-prod-helm
  chart: ./helm/charts/demo-api
  values: [helm/values-prod.yaml]
  set:
  - name: image.tag
    value: {{ env "GIT_SHA" }}
```

主な特徴:

- `helmfile diff` で全 release の差分を一括確認
- `needs:` で release 間の依存順序を制御
- `environments:` で環境ごとの values を抽象化
- Argo CD の Helmfile plugin と組み合わせて GitOps 化できる

## 選択指針

| 状況 | 推奨 |
|---|---|
| env 差分が少なく（2 環境以下）、kubectl 操作に慣れているチーム | Kustomize |
| 外部チームへの配布・release 履歴・rollback が必要 | Helm |
| 既存の OSS Chart（Prometheus, cert-manager 等）をカスタマイズする | Kustomize `helmCharts:` または Helm + post-renderer |
| 多数の Helm release をまとめて宣言管理したい | Helmfile |
| GitOps（Argo CD / Flux）環境で両方使いたい | Argo CD は Kustomize + Helm 両対応、Flux は HelmRelease CRD |

本実装では同一クラスター上に Kustomize（`demo-dev` 等 namespace）と Helm（`demo-dev-helm` 等 namespace）を並列デプロイし、同一アプリを 2 手法で管理する構成を取っている。実務では 1 アプリに 1 手法を選択し混在を避けることを推奨する。
