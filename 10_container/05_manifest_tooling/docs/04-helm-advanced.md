# 04 Helm advanced

## _helpers.tpl と named templates

`templates/_helpers.tpl` は `.yaml` 拡張子を持たないため Kubernetes に直接 apply されない。`{{- define "name" -}}` で named template を定義し、他のテンプレートから呼び出す。

```
{{- define "demo-api.fullname" -}}
{{- printf "%s-%s" .Release.Name (include "common.name" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}
```

実装は `helm/charts/demo-api/templates/_helpers.tpl` を参照。

呼び出し方は 2 種類ある。

| 関数 | 戻り値 | パイプライン |
|---|---|---|
| `{{ include "name" . }}` | 文字列（パイプラインに渡せる） | 可 |
| `{{ template "name" . }}` | void（直接出力） | 不可 |

`include` は `nindent` や `quote` などのパイプラインと組み合わせられるため、実務では `include` を優先する。`deployment.yaml` では `{{- include "common.labels" . | nindent 4 }}` のパターンを多用している。

## named templates とスコープ制御

named template に渡すコンテキスト（第 2 引数）はドット `.` または任意の値。

- `.` を渡す: 現在のスコープ全体を引き継ぐ。`.Values`, `.Release`, `.Chart` すべて参照可能
- `$` を渡す: ルートスコープを明示的に参照。`range` ループ内でルートの値が必要な場合に使う
- カスタム dict: `dict "key" .Values.foo` のように部分データを渡す

```yaml
{{- range $k, $v := .Values.env }}
- { name: {{ $k }}, value: {{ $v | quote }} }
{{- end }}
```

`range` 内では `.` がループ変数に置き換わるため、ループ外の値は `$` 経由でアクセスする（例: `{{ $.Release.Name }}`）。

## Hooks

Hooks は Kubernetes リソースに `helm.sh/hook` annotation を付与することで、release ライフサイクルの特定タイミングに実行される。

```yaml
# templates/tests/test-connection.yaml
metadata:
  annotations:
    "helm.sh/hook": test
```

| hook 種別 | タイミング |
|---|---|
| `pre-install` | install 前 |
| `post-install` | install 後 |
| `pre-upgrade` | upgrade 前 |
| `post-upgrade` | upgrade 後 |
| `pre-delete` | uninstall 前 |
| `test` | `helm test` 実行時 |

複数 hook の実行順序は `helm.sh/hook-weight` で制御し、小さい値が先に実行される。`helm.sh/hook-delete-policy` で hook Pod の削除タイミングを指定できる（`before-hook-creation` / `hook-succeeded` / `hook-failed`）。

本実装の `test-connection.yaml` は `helm.sh/hook: test` を付与した curl Pod で、`helm test` 実行時に Service への疎通確認を行う。

## Subchart / dependency / condition + tags

Chart.yaml の `dependencies:` で他の Chart を依存として宣言し、`helm dependency update` で `charts/` 以下に tgz として取得する。

```yaml
# helm/charts/demo-api/Chart.yaml
dependencies:
- name: common
  version: 0.1.0
  repository: file://../../library-chart/charts/common
```

`condition` フィールドで values のブール値と紐付けると、環境ごとに subchart を有効/無効化できる。`tags` フィールドで複数 subchart を一括切り替えることも可能。

```yaml
dependencies:
- name: redis
  version: "18.x"
  repository: https://charts.bitnami.com/bitnami
  condition: redis.enabled
  tags: [cache]
```

`helm dependency update` 実行後、`Chart.lock` に解決済みバージョンが記録され、再現性が保証される（`helm/charts/demo-api/Chart.lock` 参照）。

## Library chart

`type: library` の Chart は named template のみを提供し、単体で `helm install` できない。アプリ Chart の `dependencies:` に追加し、`include` で template を呼び出す。

```yaml
# helm/library-chart/charts/common/Chart.yaml
apiVersion: v2
name: common
version: 0.1.0
type: library
```

`common.labels` と `common.name` を定義し、`demo-api` の全テンプレートが共通ラベルと名前生成ロジックを参照している（`helm/library-chart/charts/common/templates/_labels.tpl`）。複数 Chart で同じ helper を再利用する場合に library chart を切り出す。

## values.schema.json

`values.schema.json` を Chart ルートに置くと、`helm install` / `helm upgrade` / `helm lint` 時に values の型・必須チェックが走る。JSON Schema Draft-07 に準拠。

```json
# helm/charts/demo-api/values.schema.json (抜粋)
{
  "required": ["image", "replicas"],
  "properties": {
    "replicas": { "type": "integer", "minimum": 1 },
    "image": {
      "properties": {
        "pullPolicy": { "enum": ["Always", "IfNotPresent", "Never"] }
      }
    }
  }
}
```

不正な values（`replicas: "two"` など）が渡された場合に install 前にエラーを返すため、CI で `helm lint -f helm/values-prod.yaml` を実行することで本番デプロイ前の誤設定を検出できる。
