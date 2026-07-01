# 10-5 Manifest Tooling (Kustomize + Helm) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `10_container/05_manifest_tooling/` に demo-api を Kustomize (base + overlay + components) と Helm (chart + library chart + values-per-env) の 2 手法で dev/stg/prod にレンダー、kind 上で apply → 動作確認 → `helm test`。両手法の render diff を取って機能等価性を示す。

**Architecture:** kustomize/ 側は base+3 overlays (dev/stg/prod) + 2 components (metrics/ingress) を配置。helm/ 側は demo-api chart + common library chart + values-{dev,stg,prod}.yaml。専用 kind cluster `learning-manifest` に demo-api:v1 image を kind load して両方 apply。Makefile が render/lint/apply/test/diff を全部束ねる。

**Tech Stack:** kind v0.24+, kubectl (kustomize built-in), Helm 3.15+, kubeconform, helm-diff plugin, Go 1.25+ (10-2 demo-api 再利用のためビルドで使用).

## Global Constraints

- 章 base dir: `10_container/05_manifest_tooling/`
- demo-api image: `demo-api:v1` (10-2 `../02_kind_mesh/demo-app/api/` から build)
- kind cluster: `learning-manifest` (本章専用、他章と隔離)
- Helm chart は Chart.yaml apiVersion v2、values.schema.json 必須
- Kustomize は kubectl 組込 (kustomize CLI 単体不要)
- kubeconform strict + missing-schemas 無視で全 render を通す
- Commit prefix: `feat(10-5): ...` / `docs(10-5): ...`

---

## File Structure

新規:
- `10_container/05_manifest_tooling/{README.md, Makefile, VERIFICATION.md}`
- `10_container/05_manifest_tooling/kustomize/base/{deployment.yaml, service.yaml, configmap.yaml, kustomization.yaml}`
- `10_container/05_manifest_tooling/kustomize/components/metrics/{servicemonitor.yaml, kustomization.yaml}`
- `10_container/05_manifest_tooling/kustomize/components/ingress/{ingress.yaml, kustomization.yaml}`
- `10_container/05_manifest_tooling/kustomize/overlays/{dev,stg}/kustomization.yaml`
- `10_container/05_manifest_tooling/kustomize/overlays/prod/{kustomization.yaml, patch-replicas.yaml, patch-json6902.yaml}`
- `10_container/05_manifest_tooling/helm/charts/demo-api/{Chart.yaml, values.yaml, values.schema.json, templates/{_helpers.tpl, deployment.yaml, service.yaml, configmap.yaml, hpa.yaml, ingress.yaml, NOTES.txt, tests/test-connection.yaml}}`
- `10_container/05_manifest_tooling/helm/library-chart/charts/common/{Chart.yaml, values.yaml, templates/_labels.tpl}`
- `10_container/05_manifest_tooling/helm/{values-dev.yaml, values-stg.yaml, values-prod.yaml}`
- `10_container/05_manifest_tooling/docs/{README.md, 01-kustomize-basics.md, 02-kustomize-advanced.md, 03-helm-basics.md, 04-helm-advanced.md, 05-values-strategy.md, compare.md}`

---

## Task 1: Chapter scaffold + kind cluster + demo-api image

**Files:**
- Create: `10_container/05_manifest_tooling/README.md`
- Create: `10_container/05_manifest_tooling/docs/README.md` + 6 placeholder docs

**Interfaces:**
- Produces: chapter dir, kind cluster `learning-manifest` with demo-api:v1 loaded

- [ ] **Step 1: mkdir + README**

```bash
mkdir -p 10_container/05_manifest_tooling/{kustomize/{base,components/{metrics,ingress},overlays/{dev,stg,prod}},helm/{charts/demo-api/templates/tests,library-chart/charts/common/templates},docs}
```

`10_container/05_manifest_tooling/README.md`:
```markdown
# 10-5 Manifest Tooling (Kustomize + Helm)

同じ demo-api を 2 手法で dev/stg/prod にデプロイ。

- Kustomize: `kustomize/`
- Helm: `helm/`
- 比較: `make compare`
- 検証: `make verify`
```

- [ ] **Step 2: docs skeleton (7 files)**

`docs/README.md`:
```markdown
# 10-5 docs
1. [01 Kustomize basics](./01-kustomize-basics.md)
2. [02 Kustomize advanced](./02-kustomize-advanced.md)
3. [03 Helm basics](./03-helm-basics.md)
4. [04 Helm advanced](./04-helm-advanced.md)
5. [05 Values strategy](./05-values-strategy.md)
6. [compare](./compare.md)
```

各 6 doc に `# <タイトル>\n\n(後続タスクで詳細化)\n`。

- [ ] **Step 3: kind cluster + image**

```bash
kind create cluster --name learning-manifest --wait 60s
docker build -t demo-api:v1 10_container/02_kind_mesh/demo-app/api
kind load docker-image demo-api:v1 --name learning-manifest
kubectl --context kind-learning-manifest get nodes
```

Expected: 1 Ready node、image は kind ノード内で参照可能。

- [ ] **Step 4: kubeconform + helm-diff plugin (前提 tool)**

```bash
# kubeconform (無ければ install)
command -v kubeconform || brew install kubeconform
# helm-diff plugin
helm plugin list | grep -q diff || helm plugin install https://github.com/databus23/helm-diff
```

これらは build artifact ではないので commit しない。VERIFICATION に install 手順記載。

- [ ] **Step 5: Commit**

```bash
git add 10_container/05_manifest_tooling
git commit -m "feat(10-5): scaffold chapter, docs skeleton, kind cluster setup notes"
```

---

## Task 2: Kustomize base + configmap + service + deployment

**Files:**
- Create: `10_container/05_manifest_tooling/kustomize/base/{deployment.yaml, service.yaml, configmap.yaml, kustomization.yaml}`

**Interfaces:**
- Produces: `kubectl kustomize kustomize/base` returns 3 valid resources (Deployment, Service, ConfigMap)

- [ ] **Step 1: base/deployment.yaml**

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
        image: demo-api:v1
        imagePullPolicy: IfNotPresent
        ports: [{ containerPort: 8080 }]
        env:
        - name: APP_VERSION
          valueFrom: { configMapKeyRef: { name: api-config, key: version } }
        - name: RUNTIME
          valueFrom: { configMapKeyRef: { name: api-config, key: runtime } }
        readinessProbe: { httpGet: { path: /healthz, port: 8080 }, periodSeconds: 2 }
        resources:
          requests: { cpu: 50m, memory: 64Mi }
          limits:   { cpu: 200m, memory: 128Mi }
```

- [ ] **Step 2: base/service.yaml**

```yaml
apiVersion: v1
kind: Service
metadata: { name: api }
spec:
  selector: { app: api }
  ports: [{ port: 80, targetPort: 8080 }]
```

- [ ] **Step 3: base/configmap.yaml**

```yaml
apiVersion: v1
kind: ConfigMap
metadata: { name: api-config }
data: { version: "v1", runtime: "k8s" }
```

- [ ] **Step 4: base/kustomization.yaml**

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources: [deployment.yaml, service.yaml, configmap.yaml]
commonLabels: { chapter: "10-5" }
```

- [ ] **Step 5: render + validate**

```bash
kubectl kustomize 10_container/05_manifest_tooling/kustomize/base | kubeconform -strict -ignore-missing-schemas -
```

Expected: exit 0、3 リソース validated。

- [ ] **Step 6: Commit**

```bash
git add 10_container/05_manifest_tooling/kustomize/base
git commit -m "feat(10-5): kustomize base — Deployment+Service+ConfigMap"
```

---

## Task 3: Kustomize components (metrics + ingress)

**Files:**
- Create: `10_container/05_manifest_tooling/kustomize/components/metrics/{servicemonitor.yaml, kustomization.yaml}`
- Create: `10_container/05_manifest_tooling/kustomize/components/ingress/{ingress.yaml, kustomization.yaml}`

**Interfaces:**
- Produces: 2 reusable components that overlays can `components: [...]` include

- [ ] **Step 1: components/metrics/servicemonitor.yaml**

```yaml
# Prometheus Operator が入っていない環境では kubeconform で "no schema" warn になる
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata: { name: api }
spec:
  selector: { matchLabels: { app: api } }
  endpoints: [{ port: http, interval: 15s }]
```

- [ ] **Step 2: components/metrics/kustomization.yaml**

```yaml
apiVersion: kustomize.config.k8s.io/v1alpha1
kind: Component
resources: [servicemonitor.yaml]
commonLabels: { monitoring: enabled }
```

- [ ] **Step 3: components/ingress/ingress.yaml**

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: api
  annotations: { nginx.ingress.kubernetes.io/rewrite-target: / }
spec:
  ingressClassName: nginx
  rules:
  - http:
      paths:
      - path: /api
        pathType: Prefix
        backend: { service: { name: api, port: { number: 80 } } }
```

- [ ] **Step 4: components/ingress/kustomization.yaml**

```yaml
apiVersion: kustomize.config.k8s.io/v1alpha1
kind: Component
resources: [ingress.yaml]
```

- [ ] **Step 5: 単体 render チェック (Kustomize は component 単体では render しない、overlay に載って初めて意味を持つ)**

Component は単体 apply しないため直接テストせず、Task 4 overlay で検証。ここは commit のみ。

- [ ] **Step 6: Commit**

```bash
git add 10_container/05_manifest_tooling/kustomize/components
git commit -m "feat(10-5): kustomize components — metrics (ServiceMonitor) + ingress"
```

---

## Task 4: Kustomize overlays (dev/stg/prod)

**Files:**
- Create: `10_container/05_manifest_tooling/kustomize/overlays/dev/kustomization.yaml`
- Create: `10_container/05_manifest_tooling/kustomize/overlays/stg/kustomization.yaml`
- Create: `10_container/05_manifest_tooling/kustomize/overlays/prod/{kustomization.yaml, patch-replicas.yaml, patch-json6902.yaml}`

**Interfaces:**
- Consumes: `kustomize/base`, `kustomize/components/*`
- Produces: 3 renderable overlays; dev has metrics; stg has metrics; prod has metrics + ingress + patched replicas + json6902 patch

- [ ] **Step 1: overlays/dev/kustomization.yaml**

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: demo-dev
namePrefix: dev-
resources: [../../base]
components: [../../components/metrics]
configMapGenerator:
- name: api-config
  behavior: merge
  literals: [version=v1-dev]
```

- [ ] **Step 2: overlays/stg/kustomization.yaml**

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: demo-stg
namePrefix: stg-
resources: [../../base]
components: [../../components/metrics]
configMapGenerator:
- name: api-config
  behavior: merge
  literals: [version=v1-stg]
replicas:
- name: api
  count: 2
```

- [ ] **Step 3: overlays/prod/kustomization.yaml + patches**

`overlays/prod/kustomization.yaml`:
```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: demo-prod
namePrefix: prod-
resources: [../../base]
components:
- ../../components/metrics
- ../../components/ingress
configMapGenerator:
- name: api-config
  behavior: merge
  literals: [version=v1-prod]
patches:
- path: patch-replicas.yaml
  target: { kind: Deployment, name: api }
- path: patch-json6902.yaml
  target: { kind: Deployment, name: api }
```

`overlays/prod/patch-replicas.yaml` (strategic merge):
```yaml
apiVersion: apps/v1
kind: Deployment
metadata: { name: api }
spec: { replicas: 3 }
```

`overlays/prod/patch-json6902.yaml`:
```yaml
- op: add
  path: /spec/template/spec/containers/0/env/-
  value:
    name: PROD_FLAG
    value: "true"
```

- [ ] **Step 4: render 3 overlays**

```bash
for env in dev stg prod; do
  echo "=== $env ==="
  kubectl kustomize 10_container/05_manifest_tooling/kustomize/overlays/$env | \
    kubeconform -strict -ignore-missing-schemas -
done
```

Expected: 3 render 全て exit 0。prod は 4 リソース (Deployment/Service/ConfigMap/Ingress) + ServiceMonitor 1 (warn: schema なし)。

- [ ] **Step 5: prod だけ deployment.env 内容確認**

```bash
kubectl kustomize 10_container/05_manifest_tooling/kustomize/overlays/prod | grep -A2 "PROD_FLAG"
```

Expected: `name: PROD_FLAG\n  value: "true"` を含む。

- [ ] **Step 6: Commit**

```bash
git add 10_container/05_manifest_tooling/kustomize/overlays
git commit -m "feat(10-5): kustomize overlays dev/stg/prod — replicas/components/patch (strategic+JSON6902)"
```

---

## Task 5: Helm library chart (common labels)

**Files:**
- Create: `10_container/05_manifest_tooling/helm/library-chart/charts/common/{Chart.yaml, values.yaml, templates/_labels.tpl}`

**Interfaces:**
- Produces: `common.labels` and `common.name` named templates callable from demo-api chart

- [ ] **Step 1: Chart.yaml**

```yaml
apiVersion: v2
name: common
version: 0.1.0
type: library
description: shared helper templates for chapter 10-5
```

- [ ] **Step 2: values.yaml**

```yaml
# library chart: values は通常 consumer 側で上書き。ここは空 or 例のみ。
```

- [ ] **Step 3: templates/_labels.tpl**

```
{{/* Shared labels for chapter 10-5 */}}
{{- define "common.labels" -}}
app.kubernetes.io/name: {{ include "common.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Values.image.tag | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
chapter: "10-5"
{{- end -}}

{{- define "common.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}
```

- [ ] **Step 4: helm lint (library chart)**

```bash
helm lint 10_container/05_manifest_tooling/helm/library-chart/charts/common
```

Expected: `[INFO]` のみ、error 0。

- [ ] **Step 5: Commit**

```bash
git add 10_container/05_manifest_tooling/helm/library-chart
git commit -m "feat(10-5): helm library chart with common labels/name templates"
```

---

## Task 6: Helm demo-api chart (Chart + values + schema + templates + helpers)

**Files:**
- Create: `10_container/05_manifest_tooling/helm/charts/demo-api/{Chart.yaml, values.yaml, values.schema.json}`
- Create: `10_container/05_manifest_tooling/helm/charts/demo-api/templates/{_helpers.tpl, deployment.yaml, service.yaml, configmap.yaml, hpa.yaml, ingress.yaml, NOTES.txt, tests/test-connection.yaml}`

**Interfaces:**
- Consumes: common library chart from Task 5
- Produces: renderable Helm chart passing lint + schema validation + kubeconform

- [ ] **Step 1: Chart.yaml**

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

- [ ] **Step 2: values.yaml**

```yaml
image:
  repository: demo-api
  tag: v1
  pullPolicy: IfNotPresent
replicas: 1
nameOverride: ""
service:
  type: ClusterIP
  port: 80
ingress:
  enabled: false
  className: nginx
  hosts:
  - host: api.example
    paths:
    - path: /api
      pathType: Prefix
hpa:
  enabled: false
  min: 1
  max: 5
  cpuTarget: 60
env:
  APP_VERSION: v1
  RUNTIME: k8s
resources:
  requests: { cpu: 50m, memory: 64Mi }
  limits:   { cpu: 200m, memory: 128Mi }
```

- [ ] **Step 3: values.schema.json**

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "required": ["image", "replicas"],
  "properties": {
    "image": {
      "type": "object",
      "required": ["repository", "tag"],
      "properties": {
        "repository": { "type": "string", "minLength": 1 },
        "tag":        { "type": "string", "minLength": 1 },
        "pullPolicy": { "type": "string", "enum": ["Always", "IfNotPresent", "Never"] }
      }
    },
    "replicas":     { "type": "integer", "minimum": 1 },
    "nameOverride": { "type": "string" },
    "service": {
      "type": "object",
      "properties": {
        "type": { "type": "string", "enum": ["ClusterIP", "NodePort", "LoadBalancer"] },
        "port": { "type": "integer", "minimum": 1, "maximum": 65535 }
      }
    },
    "ingress": {
      "type": "object",
      "properties": { "enabled": { "type": "boolean" } }
    },
    "hpa": {
      "type": "object",
      "properties": {
        "enabled":   { "type": "boolean" },
        "min":       { "type": "integer", "minimum": 1 },
        "max":       { "type": "integer", "minimum": 1 },
        "cpuTarget": { "type": "integer", "minimum": 1, "maximum": 100 }
      }
    },
    "env":       { "type": "object" },
    "resources": { "type": "object" }
  }
}
```

- [ ] **Step 4: templates/_helpers.tpl**

```
{{- define "demo-api.fullname" -}}
{{- printf "%s-%s" .Release.Name (include "common.name" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}
```

- [ ] **Step 5: templates/deployment.yaml**

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ include "demo-api.fullname" . }}
  labels: {{- include "common.labels" . | nindent 4 }}
spec:
  replicas: {{ .Values.replicas }}
  selector:
    matchLabels:
      app.kubernetes.io/name: {{ include "common.name" . }}
      app.kubernetes.io/instance: {{ .Release.Name }}
  template:
    metadata:
      labels: {{- include "common.labels" . | nindent 8 }}
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
        readinessProbe: { httpGet: { path: /healthz, port: 8080 }, periodSeconds: 2 }
        resources: {{- toYaml .Values.resources | nindent 10 }}
```

- [ ] **Step 6: templates/service.yaml**

```yaml
apiVersion: v1
kind: Service
metadata:
  name: {{ include "demo-api.fullname" . }}
  labels: {{- include "common.labels" . | nindent 4 }}
spec:
  type: {{ .Values.service.type }}
  ports:
  - port: {{ .Values.service.port }}
    targetPort: 8080
    protocol: TCP
    name: http
  selector:
    app.kubernetes.io/name: {{ include "common.name" . }}
    app.kubernetes.io/instance: {{ .Release.Name }}
```

- [ ] **Step 7: templates/configmap.yaml**

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ include "demo-api.fullname" . }}-config
  labels: {{- include "common.labels" . | nindent 4 }}
data:
  {{- range $k, $v := .Values.env }}
  {{ $k }}: {{ $v | quote }}
  {{- end }}
```

- [ ] **Step 8: templates/hpa.yaml**

```yaml
{{- if .Values.hpa.enabled }}
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: {{ include "demo-api.fullname" . }}
  labels: {{- include "common.labels" . | nindent 4 }}
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: {{ include "demo-api.fullname" . }}
  minReplicas: {{ .Values.hpa.min }}
  maxReplicas: {{ .Values.hpa.max }}
  metrics:
  - type: Resource
    resource:
      name: cpu
      target: { type: Utilization, averageUtilization: {{ .Values.hpa.cpuTarget }} }
{{- end }}
```

- [ ] **Step 9: templates/ingress.yaml**

```yaml
{{- if .Values.ingress.enabled }}
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: {{ include "demo-api.fullname" . }}
  labels: {{- include "common.labels" . | nindent 4 }}
spec:
  ingressClassName: {{ .Values.ingress.className }}
  rules:
  {{- range .Values.ingress.hosts }}
  - host: {{ .host }}
    http:
      paths:
      {{- range .paths }}
      - path: {{ .path }}
        pathType: {{ .pathType }}
        backend:
          service:
            name: {{ include "demo-api.fullname" $ }}
            port: { number: {{ $.Values.service.port }} }
      {{- end }}
  {{- end }}
{{- end }}
```

- [ ] **Step 10: templates/NOTES.txt**

```
demo-api chart {{ .Chart.Version }} installed as {{ .Release.Name }}.

Endpoints:
  - Service: {{ include "demo-api.fullname" . }}
  - Port: {{ .Values.service.port }}
{{- if .Values.ingress.enabled }}
  - Ingress hosts:
{{- range .Values.ingress.hosts }}
    - http://{{ .host }}{{ (index .paths 0).path }}
{{- end }}
{{- end }}

Test:
  helm test {{ .Release.Name }}
```

- [ ] **Step 11: templates/tests/test-connection.yaml**

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: "{{ include "demo-api.fullname" . }}-test"
  labels: {{- include "common.labels" . | nindent 4 }}
  annotations: { "helm.sh/hook": test }
spec:
  containers:
  - name: curl
    image: curlimages/curl:8.9.0
    args: ["curl", "-fsS", "http://{{ include "demo-api.fullname" . }}:{{ .Values.service.port }}/healthz"]
  restartPolicy: Never
```

- [ ] **Step 12: dependency update + lint + template**

```bash
cd 10_container/05_manifest_tooling/helm/charts/demo-api
helm dependency update
helm lint .
helm template demo-api . -f ../../values-dev.yaml >/dev/null    # values-dev.yaml は Task 7 で作る
```

Note: Task 7 の values-*.yaml がまだ無ければ `helm lint .` のみ実行、template は Task 7 完了後。

Expected: helm lint exit 0 (info だけ許容)。

- [ ] **Step 13: Commit**

```bash
git add 10_container/05_manifest_tooling/helm/charts/demo-api
git commit -m "feat(10-5): helm chart demo-api — Chart+values+schema+8 templates+tests"
```

---

## Task 7: Helm values per env + verify render + install + test

**Files:**
- Create: `10_container/05_manifest_tooling/helm/{values-dev.yaml, values-stg.yaml, values-prod.yaml}`

**Interfaces:**
- Consumes: chart from Task 6
- Produces: 3 env-specific values that render to valid manifests; `helm install` on dev + `helm test` succeed

- [ ] **Step 1: values-dev.yaml**

```yaml
replicas: 1
env: { APP_VERSION: v1-dev, RUNTIME: k8s }
```

- [ ] **Step 2: values-stg.yaml**

```yaml
replicas: 2
env: { APP_VERSION: v1-stg, RUNTIME: k8s }
```

- [ ] **Step 3: values-prod.yaml**

```yaml
replicas: 3
ingress:
  enabled: true
  className: nginx
  hosts:
  - host: api.example
    paths: [{ path: /api, pathType: Prefix }]
hpa:
  enabled: true
  min: 2
  max: 6
  cpuTarget: 50
env: { APP_VERSION: v1-prod, RUNTIME: k8s }
```

- [ ] **Step 4: render + validate all 3 envs**

```bash
cd 10_container/05_manifest_tooling/helm
for env in dev stg prod; do
  echo "=== $env ==="
  helm template demo-api charts/demo-api -f values-$env.yaml | kubeconform -strict -ignore-missing-schemas -
done
```

Expected: 3 render 全て exit 0。prod は Ingress + HPA 追加リソースあり。

- [ ] **Step 5: install dev + helm test**

```bash
cd 10_container/05_manifest_tooling/helm
kubectl --context kind-learning-manifest create namespace demo-dev 2>/dev/null || true
helm --kube-context kind-learning-manifest upgrade --install demo-api charts/demo-api \
  -f values-dev.yaml -n demo-dev
kubectl --context kind-learning-manifest -n demo-dev wait --for=condition=Available deploy --all --timeout=120s
helm --kube-context kind-learning-manifest test demo-api -n demo-dev
```

Expected: Deployment Available、test pod `demo-api-<...>-test` Success (curl /healthz → ok)。

- [ ] **Step 6: Commit**

```bash
git add 10_container/05_manifest_tooling/helm/values-*.yaml
git commit -m "feat(10-5): helm values-{dev,stg,prod}.yaml + install/test verified on dev"
```

---

## Task 8: Root Makefile + compare + VERIFICATION.md

**Files:**
- Create: `10_container/05_manifest_tooling/Makefile`
- Create: `10_container/05_manifest_tooling/VERIFICATION.md`

**Interfaces:**
- Consumes: Tasks 2-7 assets
- Produces: end-to-end `make verify` covering render/lint/apply/test; `make compare` diffing Kustomize vs Helm output

- [ ] **Step 1: Makefile**

```makefile
.PHONY: kustomize-render-dev kustomize-render-stg kustomize-render-prod \
        kustomize-apply-dev kustomize-diff \
        helm-lint helm-template-dev helm-template-stg helm-template-prod \
        helm-install-dev helm-test helm-diff \
        compare verify down

CTX ?= kind-learning-manifest

# ---- Kustomize ----
kustomize-render-dev:  ; kubectl kustomize kustomize/overlays/dev
kustomize-render-stg:  ; kubectl kustomize kustomize/overlays/stg
kustomize-render-prod: ; kubectl kustomize kustomize/overlays/prod

kustomize-apply-dev:
	kubectl --context $(CTX) apply -k kustomize/overlays/dev
	kubectl --context $(CTX) -n demo-dev wait --for=condition=Available deploy --all --timeout=120s

kustomize-diff:
	@echo "=== dev vs prod (kustomize) ==="
	diff <(kubectl kustomize kustomize/overlays/dev) <(kubectl kustomize kustomize/overlays/prod) || true

# ---- Helm ----
helm-lint:
	helm lint helm/charts/demo-api

helm-template-dev:  ; helm template demo-api helm/charts/demo-api -f helm/values-dev.yaml
helm-template-stg:  ; helm template demo-api helm/charts/demo-api -f helm/values-stg.yaml
helm-template-prod: ; helm template demo-api helm/charts/demo-api -f helm/values-prod.yaml

helm-install-dev:
	cd helm/charts/demo-api && helm dependency update
	kubectl --context $(CTX) create namespace demo-dev-helm 2>/dev/null || true
	helm --kube-context $(CTX) upgrade --install demo-api helm/charts/demo-api \
	  -f helm/values-dev.yaml -n demo-dev-helm
	kubectl --context $(CTX) -n demo-dev-helm wait --for=condition=Available deploy --all --timeout=120s

helm-test:
	helm --kube-context $(CTX) test demo-api -n demo-dev-helm

helm-diff:
	# helm-diff plugin 前提 (VERIFICATION に install 手順)
	helm --kube-context $(CTX) diff upgrade demo-api helm/charts/demo-api \
	  -f helm/values-prod.yaml -n demo-dev-helm || true

# ---- Compare ----
compare:
	@echo "=== kustomize prod resources ==="
	kubectl kustomize kustomize/overlays/prod | grep -E '^kind:|^  name:' | sort
	@echo ""
	@echo "=== helm prod resources ==="
	helm template demo-api helm/charts/demo-api -f helm/values-prod.yaml | grep -E '^kind:|^  name:' | sort
	@echo ""
	@echo "=== Both should render Deployment/Service/ConfigMap; helm has HPA + Ingress; kustomize has Ingress + ServiceMonitor. ==="

verify:
	kind create cluster --name learning-manifest --wait 60s 2>/dev/null || true
	docker build -t demo-api:v1 ../02_kind_mesh/demo-app/api
	kind load docker-image demo-api:v1 --name learning-manifest
	@echo "=== Kustomize render (3 envs) ==="
	$(MAKE) kustomize-render-dev  > /dev/null && echo "  dev OK"
	$(MAKE) kustomize-render-stg  > /dev/null && echo "  stg OK"
	$(MAKE) kustomize-render-prod > /dev/null && echo "  prod OK"
	@echo "=== Helm lint + render ==="
	$(MAKE) helm-lint
	$(MAKE) helm-template-dev  > /dev/null && echo "  dev OK"
	$(MAKE) helm-template-stg  > /dev/null && echo "  stg OK"
	$(MAKE) helm-template-prod > /dev/null && echo "  prod OK"
	@echo "=== Apply dev via both ==="
	$(MAKE) kustomize-apply-dev
	$(MAKE) helm-install-dev
	@echo "=== Helm test ==="
	$(MAKE) helm-test
	@echo "=== compare summary ==="
	$(MAKE) compare

down:
	-helm --kube-context $(CTX) uninstall demo-api -n demo-dev-helm
	-kubectl --context $(CTX) delete namespace demo-dev demo-dev-helm
	-kind delete cluster --name learning-manifest
```

- [ ] **Step 2: VERIFICATION.md**

```markdown
# 10-5 検証手順

## 前提
- docker + kind v0.24+
- kubectl (組込 kustomize)
- helm 3.15+ (`brew install helm`)
- kubeconform (`brew install kubeconform`)
- helm-diff plugin: `helm plugin install https://github.com/databus23/helm-diff`
- 10-2 demo-api の Dockerfile が存在 (`../02_kind_mesh/demo-app/api/`)

## 一括実行
```sh
cd 10_container/05_manifest_tooling
make verify
```

## 期待結果
- Kustomize 3 render exit 0
- Helm lint exit 0、3 template exit 0
- kustomize dev apply → Deployment Available in `demo-dev`
- helm install dev → Deployment Available in `demo-dev-helm`
- `helm test` → test pod Success (`curl /healthz → ok`)
- `make compare`: 両手法で共通 (Deployment/Service/ConfigMap) と差分 (kustomize: ServiceMonitor 追加、helm: HPA/Ingress 条件付き) が確認できる

## 使い分け早見表
- 単純な env 差分 → Kustomize (learning-curve 低)
- パラメータ豊富 / OSS 配布 → Helm (テンプレート表現力)
- 両方の強み → Helm chart を Kustomize helmCharts で消費 (hybrid、本章スコープ外、docs/compare.md 参照)

## Teardown
```sh
make down
```
```

- [ ] **Step 3: make verify 実行**

```bash
cd 10_container/05_manifest_tooling
make verify
```

Expected: exit 0、全 step OK。

- [ ] **Step 4: Commit**

```bash
git add 10_container/05_manifest_tooling/{Makefile,VERIFICATION.md}
git commit -m "feat(10-5): root Makefile with verify + compare + VERIFICATION"
```

---

## Task 9: docs 本文書き起こし (6 files)

**Files:**
- Modify: `10_container/05_manifest_tooling/docs/*.md` (6 files)

- [ ] **Step 1: 01-kustomize-basics.md**

内容 (300-500 words 目安):
- base + overlay 概念、ディレクトリ規約
- `resources`, `namePrefix`, `nameSuffix`, `namespace`, `commonLabels`, `commonAnnotations`
- patches: strategic merge / JSON6902 の差異
- `configMapGenerator` / `secretGenerator` の behavior (create/merge/replace)
- `images` field による image tag 差替 (spec: images 版 vs patches 版)
- 実装参照: `kustomize/base/`, `kustomize/overlays/`

- [ ] **Step 2: 02-kustomize-advanced.md**

内容:
- `components` (再利用可能ユニット、`apiVersion: kustomize.config.k8s.io/v1alpha1`, `kind: Component`)
- `replacements` (フィールド間参照、環境変数風に注入)
- `replicas` field 直接指定
- `transformers` (label/annotation transformer)
- 描画順序と決定性、`kustomize edit set image` 等 CLI 操作
- 実装参照: `kustomize/components/`, `overlays/prod/patch-json6902.yaml`

- [ ] **Step 3: 03-helm-basics.md**

内容:
- Chart.yaml (apiVersion v2, type application/library)
- values.yaml と `{{ .Values.X }}` 参照
- `{{ .Release }}`, `{{ .Chart }}`, `{{ .Files }}`, `{{ .Capabilities }}`
- `helm install / upgrade / uninstall / template / lint / test`
- release 名前空間、`--create-namespace`
- 実装参照: `helm/charts/demo-api/`

- [ ] **Step 4: 04-helm-advanced.md**

内容:
- `_helpers.tpl` + `{{- define "name" -}}` / `{{ include "name" . }}` / `{{ template "name" . }}` 差
- named templates と scope 制御 (`.` vs `$`)
- Hooks (`helm.sh/hook: pre-install|post-install|test`, weight, delete-policy)
- Subchart / dependency / condition + tags
- Library chart (`type: library`)
- values.schema.json による bill-of-materials
- 実装参照: `helm/library-chart/`, `helm/charts/demo-api/templates/_helpers.tpl`

- [ ] **Step 5: 05-values-strategy.md**

内容:
- env-per-file (values-dev/stg/prod.yaml)
- defaults + overrides (chart values + release-time -f/-set)
- schema validation で誤設定を install 時に弾く
- Secret 分離戦略: SealedSecrets / SOPS / External Secrets Operator (概説)
- CI で `helm template ... | kubectl diff` する GitOps 前段
- 実装参照: `helm/values-*.yaml`, `helm/charts/demo-api/values.schema.json`

- [ ] **Step 6: compare.md**

内容:
- 選択マトリクス

| 観点 | Kustomize | Helm |
|---|---|---|
| 学習曲線 | 低 | 中〜高 |
| 表現力 | 中 (patch) | 高 (Go template) |
| 型安全 | 弱 (YAML マージ) | schema.json で可 |
| OSS 配布 | 弱 | 強 (chart repository) |
| dry-run diff | kubectl diff / kustomize | helm diff plugin |
| CRD 管理 | apply 順序注意 | Chart.yaml crds/ dir |

- hybrid pattern:
  - Kustomize `helmCharts:` field で Helm chart を取込
  - Helm `--post-renderer` で Kustomize を挿入
- Helmfile (Helm chart の宣言的オーケストレーション) 概説
- 選択指針: env 差分数 / 配布要件 / チーム学習コスト

- [ ] **Step 7: grep 確認 + Commit**

```bash
grep -rn "後続タスクで詳細化" 10_container/05_manifest_tooling/docs/
```

Expected: 出力なし。

```bash
git add 10_container/05_manifest_tooling/docs
git commit -m "docs(10-5): Kustomize + Helm full guides + values strategy + compare matrix"
```

---

## Self-Review Notes

- Spec の 2 手法 (Kustomize + Helm) それぞれ base + 3 env 実装、Task 2-7 で網羅
- Task 4 の prod overlay で strategic merge + JSON6902 両方使用 → spec 準拠
- Task 6 の chart で `_helpers.tpl` + library chart 依存 + values.schema.json + `helm test` すべて含む
- Task 8 の compare は resource kind/name 列挙による粗 diff、docs/compare で trade-off 補足
- kustomize と helm は同 kind cluster の別 namespace (`demo-dev` vs `demo-dev-helm`) にデプロイ、競合回避
- helm-diff plugin と kubeconform は VERIFICATION に install 手順明記、CI での前提化はスコープ外
