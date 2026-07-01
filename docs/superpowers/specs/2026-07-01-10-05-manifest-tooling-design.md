# 10-5 Manifest Tooling (Kustomize + Helm) — 設計仕様

- 作成日: 2026-07-01
- 章: `10_container/05_manifest_tooling/`
- 関連: `10-2` (demo-api 再利用)、`10-4` (Argo CD/Flux が両手法を消費)

## 目的

Kubernetes manifest 管理の 2 大手法 (Kustomize / Helm) を「同一 demo-api を 2 手法で 3 環境 (dev/stg/prod) デプロイ」して比較学習。両者の設計思想差 (overlay vs templating) と用途別選択基準を体感する。

## スコープ

- **Kustomize**: base + overlay、strategic merge patch、JSON6902 patch、components、resource generator (configMapGenerator/secretGenerator)、replacements
- **Helm**: Chart.yaml + values + templates + `_helpers.tpl`、values.schema.json、subchart 概略、library chart、hooks、`helm test`
- **比較**: 同じ最終 manifest を両手法で生成、diff 取って機能等価性確認

スコープ外:
- Helmfile (declarative Helm orchestration) — docs 内 1 段落のみ言及
- Kustomize helmCharts field (hybrid) — docs 参考記載のみ
- Chart repository host (harbor / chart-museum) — 実運用寄り、別章余地

## アーキテクチャ

```
10_container/05_manifest_tooling/
├── docs/
│   ├── README.md
│   ├── 01-kustomize-basics.md
│   ├── 02-kustomize-advanced.md
│   ├── 03-helm-basics.md
│   ├── 04-helm-advanced.md
│   ├── 05-values-strategy.md
│   └── compare.md
├── kustomize/
│   ├── base/
│   │   ├── deployment.yaml
│   │   ├── service.yaml
│   │   ├── configmap.yaml
│   │   └── kustomization.yaml
│   ├── components/
│   │   ├── metrics/
│   │   │   ├── servicemonitor.yaml
│   │   │   └── kustomization.yaml
│   │   └── ingress/
│   │       ├── ingress.yaml
│   │       └── kustomization.yaml
│   └── overlays/
│       ├── dev/kustomization.yaml
│       ├── stg/kustomization.yaml
│       └── prod/
│           ├── kustomization.yaml
│           ├── patch-replicas.yaml
│           └── patch-json6902.yaml
├── helm/
│   ├── charts/
│   │   └── demo-api/
│   │       ├── Chart.yaml
│   │       ├── values.yaml
│   │       ├── values.schema.json
│   │       ├── templates/
│   │       │   ├── _helpers.tpl
│   │       │   ├── deployment.yaml
│   │       │   ├── service.yaml
│   │       │   ├── configmap.yaml
│   │       │   ├── hpa.yaml
│   │       │   ├── ingress.yaml
│   │       │   ├── NOTES.txt
│   │       │   └── tests/
│   │       │       └── test-connection.yaml
│   │       └── charts/  # (subchart 挿入位置、空)
│   ├── library-chart/
│   │   └── charts/common/
│   │       ├── Chart.yaml       # type: library
│   │       ├── templates/
│   │       │   └── _labels.tpl
│   │       └── values.yaml
│   ├── values-dev.yaml
│   ├── values-stg.yaml
│   └── values-prod.yaml
├── Makefile
├── README.md
└── VERIFICATION.md
```

## Kustomize (`kustomize/`)

### base/

`deployment.yaml`:
```yaml
apiVersion: apps/v1
kind: Deployment
metadata: { name: api, labels: { app: api } }
spec:
  replicas: 1
  selector: { matchLabels: { app: api } }
  template:
    metadata: { labels: { app: api } }
    spec:
      containers:
      - name: api
        image: localhost:5001/demo-api:v1
        ports: [{ containerPort: 8080 }]
        env:
        - { name: APP_VERSION, valueFrom: { configMapKeyRef: { name: api-config, key: version } } }
        readinessProbe: { httpGet: { path: /healthz, port: 8080 }, periodSeconds: 2 }
```

`configmap.yaml`, `service.yaml`, `kustomization.yaml` (`resources: [deployment.yaml, service.yaml, configmap.yaml]`).

### components/

**metrics/**: Prometheus ServiceMonitor + `metrics: enabled` label transformer:
```yaml
apiVersion: kustomize.config.k8s.io/v1alpha1
kind: Component
resources: [servicemonitor.yaml]
commonLabels: { monitoring: enabled }
```

**ingress/**: Ingress + ingressClass annotation を注入する Component。

### overlays/

**dev/kustomization.yaml**:
```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: demo-dev
resources: [../../base]
components: [../../components/metrics]
namePrefix: dev-
patches:
- target: { kind: Deployment, name: api }
  patch: |-
    - op: replace
      path: /spec/replicas
      value: 1
```

**prod/kustomization.yaml**:
```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: demo-prod
resources: [../../base]
components: [../../components/metrics, ../../components/ingress]
namePrefix: prod-
patches:
- path: patch-replicas.yaml           # strategic merge
  target: { kind: Deployment, name: api }
- path: patch-json6902.yaml           # JSON6902
  target: { kind: Deployment, name: api }
configMapGenerator:
- name: api-config
  behavior: merge
  literals: [version=v1-prod]
```

## Helm (`helm/`)

### charts/demo-api/Chart.yaml

```yaml
apiVersion: v2
name: demo-api
version: 0.1.0
appVersion: v1
description: Demo Go API for chapter 10-5 Helm learning
type: application
dependencies:
- name: common
  version: 0.1.0
  repository: file://../../library-chart/charts/common
```

### values.yaml

```yaml
image:
  repository: localhost:5001/demo-api
  tag: v1
  pullPolicy: IfNotPresent
replicas: 1
service:
  type: ClusterIP
  port: 80
ingress:
  enabled: false
  className: nginx
  hosts:
  - host: api.example
    paths: [{ path: /api, pathType: Prefix }]
hpa:
  enabled: false
  min: 1
  max: 5
  cpuTarget: 60
env:
  APP_VERSION: v1
resources:
  requests: { cpu: 50m, memory: 64Mi }
  limits:   { cpu: 200m, memory: 128Mi }
```

### values.schema.json

JSON Schema draft 7 で `image.repository` 必須、`replicas` >= 1 等を宣言。`helm install` 時に自動バリデーション。

### templates/_helpers.tpl

```
{{/* Common labels */}}
{{- define "demo-api.labels" -}}
app.kubernetes.io/name: {{ include "demo-api.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Values.image.tag | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{- define "demo-api.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}
```

### templates/deployment.yaml

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ include "demo-api.name" . }}
  labels: {{- include "demo-api.labels" . | nindent 4 }}
spec:
  replicas: {{ .Values.replicas }}
  selector:
    matchLabels: { app.kubernetes.io/name: {{ include "demo-api.name" . }} }
  template:
    metadata:
      labels: {{- include "demo-api.labels" . | nindent 8 }}
    spec:
      containers:
      - name: api
        image: "{{ .Values.image.repository }}:{{ .Values.image.tag }}"
        imagePullPolicy: {{ .Values.image.pullPolicy }}
        ports: [{ containerPort: 8080 }]
        env:
        {{- range $k, $v := .Values.env }}
        - { name: {{ $k }}, value: {{ $v | quote }} }
        {{- end }}
        resources: {{- toYaml .Values.resources | nindent 10 }}
```

### templates/tests/test-connection.yaml

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: "{{ include "demo-api.name" . }}-test"
  annotations: { "helm.sh/hook": test }
spec:
  containers:
  - name: curl
    image: curlimages/curl:8.9.0
    args: ["curl", "-fsS", "http://{{ include "demo-api.name" . }}/healthz"]
  restartPolicy: Never
```

### library-chart/charts/common/

`type: library` の chart。共通ラベル template `common.labels` を提供、`demo-api` chart が dependency として import する。実装コードは demo-api の _helpers.tpl から呼び出す形。

### values-{dev,stg,prod}.yaml

```yaml
# values-dev.yaml
replicas: 1
env: { APP_VERSION: v1-dev }

# values-stg.yaml
replicas: 2
env: { APP_VERSION: v1-stg }

# values-prod.yaml
replicas: 3
ingress: { enabled: true }
hpa: { enabled: true, min: 2, max: 6, cpuTarget: 50 }
env: { APP_VERSION: v1-prod }
```

## Makefile

```makefile
.PHONY: kustomize-render-dev kustomize-render-prod kustomize-apply-dev \
        kustomize-diff helm-lint helm-template-dev helm-template-prod \
        helm-install-dev helm-test helm-diff compare verify down

CTX ?= kind-learning-manifest

kustomize-render-dev:  ; kubectl kustomize kustomize/overlays/dev
kustomize-render-stg:  ; kubectl kustomize kustomize/overlays/stg
kustomize-render-prod: ; kubectl kustomize kustomize/overlays/prod

kustomize-apply-dev:
	kubectl --context $(CTX) apply -k kustomize/overlays/dev

kustomize-diff:
	diff <(kubectl kustomize kustomize/overlays/dev) <(kubectl kustomize kustomize/overlays/prod) || true

helm-lint:
	helm lint helm/charts/demo-api

helm-template-dev:
	helm template demo-api helm/charts/demo-api -f helm/values-dev.yaml

helm-template-prod:
	helm template demo-api helm/charts/demo-api -f helm/values-prod.yaml

helm-install-dev:
	helm dependency update helm/charts/demo-api
	helm --kube-context $(CTX) upgrade --install demo-api helm/charts/demo-api \
	  -f helm/values-dev.yaml -n demo-dev --create-namespace

helm-test:
	helm --kube-context $(CTX) test demo-api -n demo-dev

helm-diff:
	# helm-diff plugin 前提 (docs で install 手順)
	helm --kube-context $(CTX) diff upgrade demo-api helm/charts/demo-api \
	  -f helm/values-prod.yaml || true

compare:
	@echo "=== kustomize prod ==="
	kubectl kustomize kustomize/overlays/prod | kubeconform -strict -ignore-missing-schemas -
	@echo "=== helm prod ==="
	helm template demo-api helm/charts/demo-api -f helm/values-prod.yaml | kubeconform -strict -ignore-missing-schemas -
	@echo "=== diff (both render Deployment/Service/ConfigMap; env-related keys differ) ==="
	diff \
	  <(kubectl kustomize kustomize/overlays/prod | grep -E '^kind:|^  name:' | sort) \
	  <(helm template demo-api helm/charts/demo-api -f helm/values-prod.yaml | grep -E '^kind:|^  name:' | sort) || true

verify:
	kind create cluster --name learning-manifest --wait 60s || true
	docker build -t demo-api:v1 ../02_kind_mesh/demo-app/api  # 10-2 の api を再利用
	kind load docker-image demo-api:v1 --name learning-manifest
	$(MAKE) kustomize-render-dev >/dev/null
	$(MAKE) kustomize-render-stg >/dev/null
	$(MAKE) kustomize-render-prod >/dev/null
	$(MAKE) helm-lint
	$(MAKE) helm-template-dev >/dev/null
	$(MAKE) helm-template-prod >/dev/null
	$(MAKE) kustomize-apply-dev
	$(MAKE) helm-install-dev
	sleep 5
	$(MAKE) helm-test

down:
	kind delete cluster --name learning-manifest || true
```

## docs/ 構成

- `01-kustomize-basics.md`: base + overlay 概念、patches (strategic merge / JSON6902)、namePrefix / namespace / commonLabels
- `02-kustomize-advanced.md`: components、configMapGenerator/secretGenerator、replacements、vars、build 順序と決定性
- `03-helm-basics.md`: Chart.yaml / values.yaml / templates、`{{ .Values }}` `{{ .Release }}` `{{ .Chart }}` 参照、`helm install`/`helm upgrade`
- `04-helm-advanced.md`: `_helpers.tpl`、named templates、`include` vs `template`、hooks (pre-install/post-install/test)、subchart / library chart / dependency
- `05-values-strategy.md`: env-per-file、defaults + overrides、schema validation、secret 分離 (Sealed Secrets / SOPS 参考)
- `compare.md`: Kustomize vs Helm 選択表、hybrid pattern (kustomize helmCharts / helm post-render) 概略、Helmfile 概略

## 検証

- `make verify`:
  - kustomize 3 env render exit 0
  - helm lint exit 0
  - helm template 2 env exit 0 + kubeconform pass
  - kustomize dev を apply、Deployment Ready
  - helm install dev、`helm test` の test pod Success
- `make compare`: kustomize と helm の resource 一覧 (kind + name) が env 差分のみ
- VERIFICATION.md: kubeconform / helm-diff plugin install 手順、Kustomize と Helm の使い分け早見表

## テスト

- `kubeconform -strict -ignore-missing-schemas` で全出力静的検証
- `helm test` で deploy 後の疎通確認
- `helm lint` で chart スタイル違反 0

## 既存資産との関係

- demo-api: 10-2 の image (`demo-api:v1`) を local build → kind load で使用 (10-2 の kind registry `localhost:5001` を使うと learning-manifest cluster の containerd 設定が必要になるため、素直に kind load)
- Argo CD / Flux (10-4) がこの kustomize/ と helm/ ディレクトリを sync 対象にできる (10-4 spec の envs/ とは別、独立 demo)
- 10-2 の Kustomize base とは意図的に分離 (10-5 は「Kustomize/Helm 自体の学習」が主眼、10-2 は「manifest 手段としての Kustomize」)
