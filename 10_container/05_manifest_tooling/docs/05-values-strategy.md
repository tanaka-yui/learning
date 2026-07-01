# 05 Values strategy

## env-per-file

環境ごとに values ファイルを分割し、Chart の `values.yaml` をデフォルト基底として使う構成。

```
helm/
├── charts/demo-api/values.yaml   # デフォルト値（Chart 同梱）
├── values-dev.yaml               # dev 上書き
├── values-stg.yaml               # stg 上書き
└── values-prod.yaml              # prod 上書き
```

```yaml
# helm/values-dev.yaml
replicas: 1
env: { APP_VERSION: v1-dev, RUNTIME: k8s }

# helm/values-prod.yaml（抜粋）
replicas: 3
ingress:
  enabled: true
hpa:
  enabled: true
  min: 2
  max: 6
  cpuTarget: 50
```

Chart 内 `values.yaml` は全フィールドのデフォルトを定義し、env-per-file では変更点のみ記述する。これにより diff が最小化され、レビューで変更意図を把握しやすい。

## defaults + overrides の優先順位

Helm は複数の `-f` と `--set` を重ね掛け可能で、後に指定したものが優先される。

```bash
helm upgrade --install demo helm/charts/demo-api \
  -n demo-prod-helm \
  -f helm/values-prod.yaml \
  --set image.tag=$GIT_SHA    # CI で最後に差し込む
```

優先順位（低 → 高）:

1. Chart の `values.yaml`（デフォルト）
2. `-f helm/values-prod.yaml`（環境設定）
3. `--set image.tag=...`（CI 実行時の動的上書き）

`--set` は単一キーの上書きに使い、複数行の構造体を渡す場合は追加の `-f` ファイルを使う。

## schema validation で誤設定を弾く

`helm/charts/demo-api/values.schema.json` が存在する場合、`helm install` / `helm upgrade` / `helm lint` の実行時に values が JSON Schema に対してバリデーションされる。

```bash
# 型エラーがあれば install 前に失敗する
helm lint helm/charts/demo-api -f helm/values-prod.yaml

# 誤った pullPolicy を渡した場合の例
helm install demo helm/charts/demo-api --set image.pullPolicy=latest
# Error: values don't meet the specifications of the schema(s)
```

`values.schema.json` で定義している制約例（`helm/charts/demo-api/values.schema.json` 参照）：

- `replicas` は integer かつ 1 以上
- `image.pullPolicy` は `"Always" | "IfNotPresent" | "Never"` の enum
- `image.repository` と `image.tag` は必須かつ 1 文字以上

schema で検出できない論理的な誤り（`hpa.min > hpa.max` など）は `pre-install` hook や CI 側のカスタム検証で補完する。

## Secret 分離戦略

plain text の Secret 値は Git に含めない。主要な分離戦略を 3 つ示す。

**SealedSecrets（Bitnami）**
`kubeseal` CLI で Secret を暗号化した `SealedSecret` CRD に変換し、Git に push する。クラスター内の controller だけが復号できるため、リポジトリに暗号文を含めても安全。Kubernetes API に依存するため、クラスターの秘密鍵管理が重要。

**SOPS（Mozilla）**
AWS KMS / GCP KMS / Age キーで YAML / JSON を暗号化するツール。特定のフィールドだけを暗号化でき、Git diff として差分が見える（暗号化された値は変わる）。`helm secrets` plugin と組み合わせると `helm install -f secrets.enc.yaml` が可能。KMS のキー管理が前提条件。

**External Secrets Operator（ESO）**
AWS Secrets Manager / GCP Secret Manager / HashiCorp Vault 等から Secret を Pull し、Kubernetes Secret として同期する CRD ベースの operator。`ExternalSecret` / `SecretStore` リソースのみを Git 管理し、Secret 値はクラスター外のシークレットストアで管理する。三者の中で最も疎結合だが、operator のインストールが必要。

## CI で `helm template | kubectl diff` — GitOps 前段

`helm template` で生成した YAML をクラスターの現在状態と比較することで、apply 前に差分を確認できる。

```bash
# CI パイプラインでの差分確認ステップ
helm template demo helm/charts/demo-api \
  -n demo-prod-helm \
  -f helm/values-prod.yaml \
  --set image.tag=$GIT_SHA \
| kubectl diff -f - -n demo-prod-helm
```

`kubectl diff` はサーバーサイド diff を行い、実際に適用される変更のみを出力する。CI で `exit code 1`（差分あり）を「失敗」ではなくレポートとして扱い、レビュー承認後に `helm upgrade` を実行する GitOps フローが一般的。

`helm diff` plugin を使うと release の現在状態との差分を Helm のコンテキストで確認できる（`VERIFICATION.md` に install 手順あり）。Argo CD や Flux を使う場合はこのステップを diff controller が担うが、Helm のみの構成では CI での事前確認が安全弁となる。
