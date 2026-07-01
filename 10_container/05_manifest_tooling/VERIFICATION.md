# 10-5 検証手順

## 前提

| ツール | 確認コマンド | 備考 |
|---|---|---|
| docker | `docker version` | Docker Desktop / Rancher Desktop |
| kind v0.24+ | `kind version` | `brew install kind` |
| kubectl (kustomize 組込) | `kubectl version --client` | v1.27+ |
| helm 3.15+ | `helm version` | `brew install helm` |
| kubeconform | `kubeconform -v` | `brew install kubeconform` |
| helm-diff plugin | `helm plugin list` | 下記参照 |
| demo-api Dockerfile | `ls ../02_kind_mesh/demo-app/api/Dockerfile` | 10-2 成果物 |

### helm-diff プラグインのインストール

```sh
helm plugin install https://github.com/databus23/helm-diff
```

### PATH / ツールパス上書き

このプロジェクトの Makefile は `HELM ?= helm` / `KUBECONFORM ?= kubeconform` で
デフォルトを PATH 参照にしている。Rancher Desktop など PATH に入らない環境では
`make` 変数で明示的に上書きできる:

```sh
make HELM=/Users/yui/.rd/bin/helm helm-lint
make KUBECONFORM=/Users/yui/.local/bin/kubeconform verify
```

全コマンドを上書きして一括実行する場合:

```sh
make HELM=/Users/yui/.rd/bin/helm KUBECONFORM=/Users/yui/.local/bin/kubeconform verify
```

---

## 一括実行

```sh
cd 10_container/05_manifest_tooling
make verify
```

---

## ステップ別実行

### 1. Kustomize — 3 環境レンダリング

```sh
make kustomize-render-dev
make kustomize-render-stg
make kustomize-render-prod
```

期待: 各コマンドが YAML を stdout に出力し exit 0。

### 2. Kustomize — dev/prod 差分確認

```sh
make kustomize-diff
```

期待: `diff` の出力が表示される (差分がなければ空)。exit 0 (diff の exit code は `|| true` で無視)。

### 3. Helm lint

```sh
make helm-lint
```

期待: `1 chart(s) linted, 0 chart(s) failed` が出力され exit 0。

### 4. Helm — 3 環境テンプレート展開

```sh
make helm-template-dev
make helm-template-stg
make helm-template-prod
```

期待: 各コマンドが YAML を stdout に出力し exit 0。

### 5. kind クラスター作成 + イメージロード

`make verify` 内で自動実行されるが、手動で行う場合:

```sh
kind create cluster --name learning-manifest --wait 60s
docker build -t demo-api:v1 ../02_kind_mesh/demo-app/api
kind load docker-image demo-api:v1 --name learning-manifest
```

### 6. Kustomize dev apply

```sh
make kustomize-apply-dev
```

期待: `demo-dev` namespace に Deployment が Available になる。

### 7. Helm dev install

```sh
make helm-install-dev
```

期待: `demo-dev-helm` namespace に Deployment が Available になる。

### 8. Helm test

```sh
make helm-test
```

期待: `test-connection` Pod が Succeeded し、`curl /healthz → ok` の応答が確認される。

### 9. compare サマリ

```sh
make compare
```

期待出力例:

```
=== kustomize prod resources ===
  name: prod-api-config
  name: prod-demo-api
  name: prod-demo-api
kind: ConfigMap
kind: Deployment
kind: Ingress
kind: Service
kind: ServiceMonitor

=== helm prod resources ===
  name: demo-api
  name: demo-api
  name: demo-api
  name: demo-api
  name: demo-api
kind: ConfigMap
kind: Deployment
kind: HorizontalPodAutoscaler
kind: Ingress
kind: Service

=== Both should render Deployment/Service/ConfigMap; helm has HPA + Ingress; kustomize has Ingress + ServiceMonitor. ===
```

---

## 期待結果まとめ

| ステップ | 期待 |
|---|---|
| Kustomize 3 render | exit 0、各環境 YAML 出力 |
| Helm lint | exit 0、`0 chart(s) failed` |
| Helm 3 template | exit 0、各環境 YAML 出力 |
| kustomize-apply-dev | `demo-dev` Deployment Available |
| helm-install-dev | `demo-dev-helm` Deployment Available |
| helm test | test pod Succeeded、`/healthz → ok` |
| compare | 共通リソース確認、ツール間差分確認 |

---

## 使い分け早見表

| シナリオ | 推奨 | 理由 |
|---|---|---|
| 単純な env 差分 (replica 数・image tag) | **Kustomize** | learning-curve が低い、YAML パッチで直感的 |
| パラメータ豊富 / OSS 配布 | **Helm** | テンプレート表現力、`helm test` / `helm diff` |
| GitOps (Argo CD / Flux) | **どちらも対応** | Argo CD は両方ネイティブサポート |
| 両方の強み | **Helm chart を Kustomize helmCharts で消費 (hybrid)** | 本章スコープ外: `docs/compare.md` 参照 |

---

## helm-diff が使えない場合

`helm plugin list` で `diff` が表示されない場合、`make helm-diff` は `|| true` で
失敗を無視して終了する。`make verify` はこのターゲットを呼ばないため
全体への影響はない。

手動でインストール後に再実行:

```sh
helm plugin install https://github.com/databus23/helm-diff
make helm-diff
```

---

## 既知の制限事項

### kustomize-apply-dev — ServiceMonitor CRD

bare kind クラスターには prometheus-operator CRDs が含まれないが、
`make verify` は `kustomize-apply-dev` の前に `install-servicemonitor-crd` を
自動実行するため手動対応は不要になった。

手動で apply する場合も `make install-servicemonitor-crd` を先に実行すればよい:

```sh
make install-servicemonitor-crd
make kustomize-apply-dev
```

### Helm ライブラリチャートの更新

`helm/charts/demo-api/` が依存するライブラリチャートを変更した場合は、
`helm dependency update` を再実行して `Chart.lock` を更新すること:

```sh
helm dependency update helm/charts/demo-api/
```

生成される `.tgz` は `helm/charts/demo-api/charts/.gitignore` により
以降のコミットから除外される (初回コミット済みの `common-0.1.0.tgz` はそのまま残す)。

---

## Teardown

```sh
make down
```

実行内容:
1. `helm uninstall demo-api -n demo-dev-helm`
2. `kubectl delete namespace demo-dev demo-dev-helm`
3. `kind delete cluster --name learning-manifest`

各コマンドはリソースが存在しなくてもエラーにならない (`-` プレフィックス)。
