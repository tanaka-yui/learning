# 02 Kustomize advanced

## components

`components` は複数の overlay で共有できる再利用単位で、`apiVersion: kustomize.config.k8s.io/v1alpha1` / `kind: Component` で定義する。overlay の `resources:` ではなく `components:` フィールドで取り込む点が異なる。

```yaml
# kustomize/components/metrics/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1alpha1
kind: Component
resources: [servicemonitor.yaml]
commonLabels: { monitoring: enabled }
```

```yaml
# kustomize/components/ingress/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1alpha1
kind: Component
resources: [ingress.yaml]
```

dev・stg overlay は `metrics` component のみを取り込み、prod overlay は `metrics` と `ingress` の両方を取り込む（`kustomize/overlays/prod/kustomization.yaml`）。

`resources:` として追加すると overlay 単体で独立した kustomization が必要になるが、`components:` を使うことで「機能の有無」を宣言的に切り替えられる。

## replacements

`replacements` は、あるリソースのフィールド値を別リソースのフィールドへ自動コピーするしくみ。ConfigMap や Secret の値をコンテナの環境変数パスへ注入するユースケースで使う。

```yaml
replacements:
- source:
    kind: ConfigMap
    name: api-config
    fieldPath: data.version
  targets:
  - select: { kind: Deployment, name: api }
    fieldPaths: [spec.template.spec.containers.0.env.0.value]
```

patches で手書きするよりも参照元が明確になり、値変更が一箇所で完結する。本実装では ConfigMap → Deployment の参照を env の `valueFrom.configMapKeyRef` で行っているが、`replacements` への移行も同等の効果を得られる。

## replicas フィールド

`replicas` フィールドを使うと、patches を書かずに Deployment のレプリカ数を上書きできる。

```yaml
# kustomize/overlays/stg/kustomization.yaml
replicas:
- name: api
  count: 2
```

prod overlay では strategic merge patch（`kustomize/overlays/prod/patch-replicas.yaml`）で `replicas: 3` を設定しており、`replicas` フィールドと patches の両方が同等の機能を提供する。`replicas` フィールドの方が意図が明確で記述量が少ない。

## transformers

`transformers` は label / annotation の付与など、Kustomize 組み込みの変換処理を外部設定で制御する。`labelTransformer` や `annotationsTransformer` を YAML ファイルに定義して `transformers:` で参照する。

```yaml
apiVersion: builtin
kind: LabelTransformer
metadata: { name: labels }
labels: { env: prod }
fieldSpecs:
- path: metadata/labels
  create: true
- path: spec/template/metadata/labels
  create: true
  kind: Deployment
```

`commonLabels` が `spec.selector` にも触れてしまう問題を回避したい場合、`transformers` で `fieldSpecs` を明示的に制限する手法が有効。

## 描画順序と決定性

`kustomize build` の出力順序は入力ファイルの列挙順に依存するが、同一ファイル内のリソース順序は保証される。`resources:` の記述順がそのまま出力順になるため、CRD → CR の順序制約がある場合は `resources:` 内で上に書く。

`kustomize build` は冪等で、同一入力に対して常に同一出力を生成する（ハッシュ付き ConfigMap 名は内容依存）。

## CLI 操作

```bash
# イメージタグを書き換えて kustomization.yaml を更新
kustomize edit set image demo-api=demo-api:v2

# overlay をビルドして確認
kustomize build kustomize/overlays/prod

# kubectl に直接パイプ
kustomize build kustomize/overlays/prod | kubectl apply -f -

# kubectl 組み込み (-k フラグ)
kubectl apply -k kustomize/overlays/prod
kubectl diff -k kustomize/overlays/prod
```

`kustomize edit set image` は `kustomization.yaml` の `images:` フィールドを in-place 更新するため、CI パイプラインでイメージタグをコミットに応じて動的に差し替えるフローに適している（`overlays/prod/kustomization.yaml` を CI が直接書き換える GitOps パターン）。
