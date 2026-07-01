# 03 Helm basics

## Chart.yaml

Chart のメタデータを定義するファイル。`apiVersion: v2` は Helm 3 以降の形式。

```yaml
# helm/charts/demo-api/Chart.yaml
apiVersion: v2
name: demo-api
version: 0.1.0        # Chart バージョン（SemVer 2）
appVersion: v1        # アプリケーション バージョン（任意の文字列）
description: Demo Go API for chapter 10-5 Helm learning
type: application     # application | library
dependencies:
- name: common
  version: 0.1.0
  repository: file://../../library-chart/charts/common
```

`type: application` は通常の Chart。`type: library` は named template のみを提供し、単体インストールできない（後述）。`version` は Chart 自体のバージョンで、`appVersion` はデプロイ対象アプリのバージョンを表す。

## values.yaml と .Values 参照

`values.yaml` は Chart のデフォルト設定値ファイル。テンプレート内では `{{ .Values.キー }}` で参照する。

```yaml
# helm/charts/demo-api/values.yaml (抜粋)
image:
  repository: demo-api
  tag: v1
replicas: 1
service:
  type: ClusterIP
  port: 80
```

```yaml
# templates/deployment.yaml (抜粋)
image: "{{ .Values.image.repository }}:{{ .Values.image.tag }}"
replicas: {{ .Values.replicas }}
```

ネストしたキーはドットでアクセスする（`{{ .Values.image.repository }}`）。`helm install -f` または `helm install --set` でデフォルト値を上書きできる。

## 組み込みオブジェクト

| オブジェクト | 主なフィールド | 用途 |
|---|---|---|
| `.Release` | `.Name`, `.Namespace`, `.Service`, `.IsInstall`, `.IsUpgrade` | release コンテキスト |
| `.Chart` | `.Name`, `.Version`, `.AppVersion` | Chart.yaml の値 |
| `.Files` | `.Get "path"`, `.Glob "pattern"` | Chart 内の任意ファイル読み込み |
| `.Capabilities` | `.KubeVersion.GitVersion`, `.APIVersions.Has` | クラスター capability 確認 |

`templates/deployment.yaml` では `.Release.Name` を fullname に埋め込み、`.Values.image.tag` をイメージタグに参照している。`.Capabilities` は HPA や Ingress の apiVersion 分岐などに使う。

## 主要コマンド

```bash
# Chart インストール（release 名 + Chart パス）
helm install demo helm/charts/demo-api -n demo-dev-helm --create-namespace

# 差分適用（存在しなければ install）
helm upgrade --install demo helm/charts/demo-api -n demo-dev-helm \
  -f helm/values-dev.yaml

# release 削除（namespace は残る）
helm uninstall demo -n demo-dev-helm

# YAML レンダリング（クラスター不要）
helm template demo helm/charts/demo-api -f helm/values-prod.yaml

# 構文・スキーマ検証
helm lint helm/charts/demo-api -f helm/values-prod.yaml

# テスト Pod 実行
helm test demo -n demo-dev-helm
```

`helm template` は apply せずに生成 YAML を stdout に出力するため、`kubectl diff` との組み合わせや CI での事前確認に使う。

## release 名前空間と --create-namespace

Helm の release は名前空間に紐付く。`helm install` 時に `-n <ns>` で指定し、存在しない場合は `--create-namespace` を併用して自動作成する。`helm list -n <ns>` で名前空間内の release 一覧を確認できる。

本実装では dev / stg / prod それぞれ `demo-dev-helm` / `demo-stg-helm` / `demo-prod-helm` 名前空間を使用し、Kustomize 側の `demo-dev` 等と名前空間を分離している（`Makefile` の `helm-install-*` ターゲット参照）。
