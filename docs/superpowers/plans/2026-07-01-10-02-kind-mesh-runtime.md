# 10-2 kind + 最新コンテナエコシステム Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** kind 上で動く demo-app (Go API + Node Web + Spin/WASM 版) を base manifest として用意し、Istio (sidecar + ambient), Envoy Gateway, Linkerd, Cilium, SpinKube, Kata/gVisor, vCluster, Karpenter (kwok) の overlay を Kustomize で重ねて切替検証可能にする。

**Architecture:** 3 kind cluster (`learning-base`, `learning-linkerd`, `learning-cilium`)。base に Istio (ambient profile)/Envoy Gateway/SpinKube/Kata/vCluster/Karpenter (kwok provider) を同居。Linkerd と Cilium は別 cluster で隔離。Kustomize overlay で manifest を共有、`make install-<tech>` で順に投入。

**Tech Stack:** kind v0.24+, kubectl, Kustomize, Helm, Istio 1.24 (ambient profile), Envoy Gateway v1.2, Linkerd edge, Cilium 1.16, SpinKube (spin-operator + cert-manager), kata-deploy, vcluster v0.20, Karpenter OSS + kwok provider, Go 1.26, Node 22 (Fastify), Spin (Rust → wasm32-wasi).

## Global Constraints

- Base ディレクトリ: `10_container/02_kind_mesh/`
- Container registry: local kind registry (`localhost:5001`)
- Demo image tags: `localhost:5001/demo-api:v1`, `:v2`, `localhost:5001/demo-web:latest`, `localhost:5001/demo-spin:v1`
- demo-api コード: 10-1 の B パターンを流用 (build pattern + non-root + distroless static)
- Istio: ambient profile でインストール、sidecar overlay は namespace label で同居
- 別 cluster: `kubectl config use-context kind-<cluster-name>` を明示的に切り替える
- 全 manifest は `kubeconform -strict -ignore-missing-schemas` 通過

---

## File Structure

新規作成:
- `10_container/02_kind_mesh/README.md`
- `10_container/02_kind_mesh/VERIFICATION.md`
- `10_container/02_kind_mesh/Makefile`
- `10_container/02_kind_mesh/kind/{cluster.yaml, cluster-linkerd.yaml, cluster-cilium.yaml, bootstrap.sh, registry.sh}`
- `10_container/02_kind_mesh/demo-app/api/{main.go, main_test.go, go.mod, Dockerfile}`
- `10_container/02_kind_mesh/demo-app/web/{server.js, package.json, server.test.js, Dockerfile}`
- `10_container/02_kind_mesh/demo-app/spin/{spin.toml, src/lib.rs, Cargo.toml, Dockerfile}`
- `10_container/02_kind_mesh/manifests/base/{api-v1.yaml, api-v2.yaml, web.yaml, hpa.yaml, kustomization.yaml}`
- `10_container/02_kind_mesh/manifests/overlays/{istio-sidecar, istio-ambient, envoy-gateway, linkerd, cilium, spinkube, kata, vcluster, karpenter}/kustomization.yaml` 他
- `10_container/02_kind_mesh/docs/{README.md, 01-kind-basics.md, 02-istio-sidecar.md, 03-istio-ambient.md, 04-envoy-gateway.md, 05-linkerd.md, 06-cilium-mesh.md, 07-spinkube-wasm.md, 08-kata-gvisor.md, 09-vcluster.md, 10-karpenter.md, compare.md}`

---

## Task 1: Chapter scaffold + docs skeleton + kind cluster.yaml

**Files:**
- Create: `10_container/02_kind_mesh/{README.md, kind/{cluster.yaml,cluster-linkerd.yaml,cluster-cilium.yaml,bootstrap.sh,registry.sh}}`
- Create: `10_container/02_kind_mesh/docs/{README.md ほか 11 ファイル}` (見出しのみ)

**Interfaces:**
- Produces: `./kind/bootstrap.sh` で base cluster + local registry が立つ

- [ ] **Step 1: ディレクトリ作成**

```bash
mkdir -p 10_container/02_kind_mesh/{kind,demo-app/{api,web,spin/src},manifests/{base,overlays/{istio-sidecar,istio-ambient,envoy-gateway,linkerd,cilium,spinkube,kata,vcluster,karpenter}},docs}
```

- [ ] **Step 2: 章 README.md**

```markdown
# 10-2 kind + 最新コンテナエコシステム

| 技術 | 学習対象 | overlay |
|---|---|---|
| Istio (sidecar/ambient) | mTLS, traffic shift, waypoint | istio-sidecar / istio-ambient |
| Envoy Gateway | Gateway API | envoy-gateway |
| Linkerd | Rust micro-proxy, SMI | linkerd |
| Cilium | eBPF, L7 NetworkPolicy, Hubble | cilium |
| SpinKube | WASM workload | spinkube |
| Kata/gVisor | Sandbox runtime, RuntimeClass | kata |
| vCluster | 仮想クラスタ | vcluster |
| Karpenter (OSS) | NodePool / kwok provider | karpenter |

検証: `make verify`。詳細: `docs/`, `VERIFICATION.md`。
```

- [ ] **Step 3: kind/cluster.yaml (base)**

```yaml
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
name: learning-base
nodes:
- role: control-plane
  kubeadmConfigPatches:
  - |
    kind: InitConfiguration
    nodeRegistration:
      kubeletExtraArgs:
        node-labels: "ingress-ready=true"
  extraPortMappings:
  - { containerPort: 80,  hostPort: 80,  protocol: TCP }
  - { containerPort: 443, hostPort: 443, protocol: TCP }
- role: worker
- role: worker
containerdConfigPatches:
- |-
  [plugins."io.containerd.grpc.v1.cri".registry]
    config_path = "/etc/containerd/certs.d"
```

- [ ] **Step 4: kind/cluster-linkerd.yaml と cluster-cilium.yaml**

`cluster-linkerd.yaml`:

```yaml
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
name: learning-linkerd
nodes:
- role: control-plane
- role: worker
```

`cluster-cilium.yaml`:

```yaml
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
name: learning-cilium
networking:
  disableDefaultCNI: true       # Cilium で置換
  kubeProxyMode: none           # Cilium が代替
nodes:
- role: control-plane
- role: worker
```

- [ ] **Step 5: kind/registry.sh + bootstrap.sh**

`registry.sh` (kind 公式の local registry レシピ準拠):

```sh
#!/bin/sh
set -e
REG_NAME='kind-registry'
REG_PORT='5001'
if [ "$(docker inspect -f '{{.State.Running}}' "$REG_NAME" 2>/dev/null || true)" != 'true' ]; then
  docker run -d --restart=always -p "127.0.0.1:${REG_PORT}:5000" --network bridge --name "$REG_NAME" registry:2
fi
```

`bootstrap.sh`:

```sh
#!/bin/sh
set -e
./kind/registry.sh
kind create cluster --config kind/cluster.yaml || true
# containerd に registry mirror 設定を撒く
for node in $(kind get nodes --name learning-base); do
  docker exec "$node" mkdir -p /etc/containerd/certs.d/localhost:5001
  cat <<EOF | docker exec -i "$node" tee /etc/containerd/certs.d/localhost:5001/hosts.toml >/dev/null
[host."http://kind-registry:5000"]
EOF
done
# kind network に registry を join
docker network connect "kind" kind-registry 2>/dev/null || true
kubectl apply -f - <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: local-registry-hosting
  namespace: kube-public
data:
  localRegistryHosting.v1: |
    host: "localhost:5001"
    help: "https://kind.sigs.k8s.io/docs/user/local-registry/"
EOF
```

```bash
chmod +x 10_container/02_kind_mesh/kind/*.sh
```

- [ ] **Step 6: docs スケルトン (12 ファイル)**

各 `docs/*.md` に H1 と 1 行説明だけ書く。本文は Task 15 で書く。

- [ ] **Step 7: bootstrap 検証**

```bash
cd 10_container/02_kind_mesh
./kind/bootstrap.sh
kubectl --context kind-learning-base get nodes
docker ps | grep kind-registry
```

Expected: 3 ノード Ready、registry コンテナが Up。

- [ ] **Step 8: Commit**

```bash
git add 10_container/02_kind_mesh
git commit -m "feat(10-2): scaffold chapter, kind clusters, local registry bootstrap"
```

---

## Task 2: demo-app/api (Go)

**Files:**
- Create: `10_container/02_kind_mesh/demo-app/api/{main.go,main_test.go,go.mod,Dockerfile}`

**Interfaces:**
- Produces:
  - `GET /api/v1/echo` / `GET /api/v2/echo` → `{"version":"v1|v2","host":"<pod>"}`
  - `GET /healthz` → `ok`
  - `GET /admin/secret` → `{"secret":"shhh"}` (Cilium L7 policy で deny される対象)
  - image: `localhost:5001/demo-api:v1` と `:v2` (build arg `APP_VERSION` で切替)

- [ ] **Step 1: failing test**

`demo-app/api/main_test.go`:

```go
package main

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestEchoVersionFromEnv(t *testing.T) {
	t.Setenv("APP_VERSION", "v9")
	w := httptest.NewRecorder()
	echo(w, httptest.NewRequest("GET", "/api/v1/echo", nil))
	if w.Code != 200 {
		t.Fatalf("want 200, got %d", w.Code)
	}
	var got map[string]string
	_ = json.NewDecoder(w.Result().Body).Decode(&got)
	if got["version"] != "v9" {
		t.Fatalf("want v9, got %s", got["version"])
	}
}
```

- [ ] **Step 2: 失敗確認**

```bash
cd 10_container/02_kind_mesh/demo-app/api
go mod init github.com/yui/learning/10/02/api
go test ./...
```

Expected: `undefined: echo` で FAIL。

- [ ] **Step 3: main.go**

```go
package main

import (
	"encoding/json"
	"net/http"
	"os"
)

func echo(w http.ResponseWriter, _ *http.Request) {
	host, _ := os.Hostname()
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"version": os.Getenv("APP_VERSION"),
		"host":    host,
	})
}

func secret(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"secret": "shhh"})
}

func main() {
	http.HandleFunc("/api/v1/echo", echo)
	http.HandleFunc("/api/v2/echo", echo)
	http.HandleFunc("/admin/secret", secret)
	http.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok")) })
	if err := http.ListenAndServe(":8080", nil); err != nil {
		panic(err)
	}
}
```

- [ ] **Step 4: test pass**

```bash
go test ./...
```

Expected: PASS。

- [ ] **Step 5: Dockerfile (10-1 B パターン流用)**

```dockerfile
# syntax=docker/dockerfile:1.7
ARG APP_VERSION=v1
FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/api .

FROM gcr.io/distroless/static-debian12
ARG APP_VERSION
ENV APP_VERSION=${APP_VERSION}
COPY --from=build /out/api /api
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/api"]
```

- [ ] **Step 6: image 2 種を build + push**

```bash
cd 10_container/02_kind_mesh
docker build --build-arg APP_VERSION=v1 -t localhost:5001/demo-api:v1 demo-app/api
docker build --build-arg APP_VERSION=v2 -t localhost:5001/demo-api:v2 demo-app/api
docker push localhost:5001/demo-api:v1
docker push localhost:5001/demo-api:v2
```

Expected: push 成功。

- [ ] **Step 7: smoke (image 動作確認)**

```bash
docker run --rm -d -p 8080:8080 --name api-smoke localhost:5001/demo-api:v1
sleep 1
curl -fsS http://localhost:8080/api/v1/echo
curl -fsS http://localhost:8080/admin/secret
docker rm -f api-smoke
```

Expected: `{"version":"v1",...}` と `{"secret":"shhh"}`。

- [ ] **Step 8: Commit**

```bash
git add 10_container/02_kind_mesh/demo-app/api
git commit -m "feat(10-2): demo-app api (Go) with v1/v2 + admin endpoint for L7 policy"
```

---

## Task 3: demo-app/web (Node Fastify)

**Files:**
- Create: `10_container/02_kind_mesh/demo-app/web/{server.js,server.test.js,package.json,Dockerfile}`

**Interfaces:**
- Produces:
  - `GET /` → api を 10 回呼んで version 分布を JSON 返却
  - image: `localhost:5001/demo-web:latest`

- [ ] **Step 1: failing test**

`web/server.test.js`:

```js
const { tallyVersions } = require('./server')

test('tallyVersions counts v1/v2', () => {
  const result = tallyVersions([{ version: 'v1' }, { version: 'v2' }, { version: 'v1' }])
  expect(result).toEqual({ v1: 2, v2: 1 })
})
```

- [ ] **Step 2: package.json**

```json
{
  "name": "demo-web",
  "private": true,
  "scripts": { "test": "node --test server.test.js", "start": "node server.js" },
  "dependencies": { "fastify": "^4.28.1" }
}
```

- [ ] **Step 3: 失敗確認**

```bash
cd 10_container/02_kind_mesh/demo-app/web
pnpm install
pnpm test
```

Expected: `Cannot find module './server'` で FAIL。

- [ ] **Step 4: server.js**

```js
const Fastify = require('fastify')

function tallyVersions(responses) {
  return responses.reduce((acc, r) => {
    acc[r.version] = (acc[r.version] || 0) + 1
    return acc
  }, {})
}

async function fetchEcho(url) {
  const res = await fetch(url + '/api/v1/echo')
  return res.json()
}

const app = Fastify()
app.get('/healthz', async () => 'ok')
app.get('/', async () => {
  const target = process.env.API_URL || 'http://api'
  const calls = await Promise.all(Array.from({ length: 10 }, () => fetchEcho(target)))
  return tallyVersions(calls)
})

if (require.main === module) {
  app.listen({ host: '0.0.0.0', port: 3000 }).catch((err) => { console.error(err); process.exit(1) })
}

module.exports = { tallyVersions }
```

- [ ] **Step 5: test pass**

```bash
pnpm test
```

Expected: PASS。

- [ ] **Step 6: Dockerfile**

```dockerfile
# syntax=docker/dockerfile:1.7
FROM node:22-alpine AS build
WORKDIR /src
COPY package.json pnpm-lock.yaml* ./
RUN --mount=type=cache,target=/root/.local/share/pnpm/store \
    corepack enable && pnpm install --frozen-lockfile --prod

FROM node:22-alpine
WORKDIR /app
COPY --from=build /src/node_modules ./node_modules
COPY server.js ./
ENV NODE_ENV=production
USER node
EXPOSE 3000
CMD ["node", "server.js"]
```

- [ ] **Step 7: image build + push**

```bash
cd 10_container/02_kind_mesh
docker build -t localhost:5001/demo-web:latest demo-app/web
docker push localhost:5001/demo-web:latest
```

- [ ] **Step 8: Commit**

```bash
git add 10_container/02_kind_mesh/demo-app/web
git commit -m "feat(10-2): demo-app web (Fastify) tallying api version distribution"
```

---

## Task 4: demo-app/spin (Rust → wasm)

**Files:**
- Create: `10_container/02_kind_mesh/demo-app/spin/{spin.toml,Cargo.toml,src/lib.rs}`

**Interfaces:**
- Produces: SpinApp 用 wasm module を OCI artifact として registry に push (`localhost:5001/demo-spin:v1`)

- [ ] **Step 1: Cargo.toml**

```toml
[package]
name = "demo-spin"
version = "0.1.0"
edition = "2021"

[lib]
crate-type = ["cdylib"]

[dependencies]
spin-sdk = "3.0"
serde = { version = "1", features = ["derive"] }
serde_json = "1"
```

- [ ] **Step 2: src/lib.rs**

```rust
use spin_sdk::http::{IntoResponse, Request, Response};
use spin_sdk::http_component;

#[http_component]
fn handle(_req: Request) -> anyhow::Result<impl IntoResponse> {
    let body = serde_json::json!({ "version": "v1-wasm", "runtime": "spin" });
    Ok(Response::builder()
        .status(200)
        .header("content-type", "application/json")
        .body(serde_json::to_vec(&body)?)
        .build())
}
```

- [ ] **Step 3: spin.toml**

```toml
spin_manifest_version = 2
[application]
name = "demo-spin"
version = "0.1.0"

[[trigger.http]]
route = "/api/v1/echo"
component = "demo-spin"

[component.demo-spin]
source = "target/wasm32-wasi/release/demo_spin.wasm"
[component.demo-spin.build]
command = "cargo build --target wasm32-wasi --release"
```

- [ ] **Step 4: build + push OCI artifact**

```bash
cd 10_container/02_kind_mesh/demo-app/spin
rustup target add wasm32-wasi
spin build
spin registry push localhost:5001/demo-spin:v1 --insecure
```

注: 環境に `spin` CLI 未導入なら `brew install fermyon/tap/spin` 等で導入する旨を docs に書く。

- [ ] **Step 5: Commit**

```bash
git add 10_container/02_kind_mesh/demo-app/spin
git commit -m "feat(10-2): demo-app spin (Rust → wasm) component + OCI artifact"
```

---

## Task 5: manifests/base (Kustomize)

**Files:**
- Create: `10_container/02_kind_mesh/manifests/base/{api-v1.yaml,api-v2.yaml,web.yaml,hpa.yaml,kustomization.yaml}`

**Interfaces:**
- Produces:
  - Deployment `api-v1`, `api-v2` (各 replicas:2), Service `api` (両者を label `app=api` で集約)
  - Deployment `web` + Service `web`
  - HPA を `web` に付与 (CPU 50%)

- [ ] **Step 1: base/api-v1.yaml**

```yaml
apiVersion: apps/v1
kind: Deployment
metadata: { name: api-v1, labels: { app: api, version: v1 } }
spec:
  replicas: 2
  selector: { matchLabels: { app: api, version: v1 } }
  template:
    metadata: { labels: { app: api, version: v1 } }
    spec:
      containers:
      - name: api
        image: localhost:5001/demo-api:v1
        ports: [{ containerPort: 8080 }]
        readinessProbe: { httpGet: { path: /healthz, port: 8080 }, periodSeconds: 2 }
---
apiVersion: v1
kind: Service
metadata: { name: api }
spec:
  selector: { app: api }
  ports: [{ port: 80, targetPort: 8080 }]
```

- [ ] **Step 2: base/api-v2.yaml**

```yaml
apiVersion: apps/v1
kind: Deployment
metadata: { name: api-v2, labels: { app: api, version: v2 } }
spec:
  replicas: 2
  selector: { matchLabels: { app: api, version: v2 } }
  template:
    metadata: { labels: { app: api, version: v2 } }
    spec:
      containers:
      - name: api
        image: localhost:5001/demo-api:v2
        ports: [{ containerPort: 8080 }]
        readinessProbe: { httpGet: { path: /healthz, port: 8080 }, periodSeconds: 2 }
```

- [ ] **Step 3: base/web.yaml**

```yaml
apiVersion: apps/v1
kind: Deployment
metadata: { name: web, labels: { app: web } }
spec:
  replicas: 1
  selector: { matchLabels: { app: web } }
  template:
    metadata: { labels: { app: web } }
    spec:
      containers:
      - name: web
        image: localhost:5001/demo-web:latest
        env:
        - { name: API_URL, value: "http://api" }
        ports: [{ containerPort: 3000 }]
        resources:
          requests: { cpu: 50m, memory: 64Mi }
          limits:   { cpu: 200m, memory: 128Mi }
---
apiVersion: v1
kind: Service
metadata: { name: web }
spec:
  selector: { app: web }
  ports: [{ port: 80, targetPort: 3000 }]
```

- [ ] **Step 4: base/hpa.yaml**

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata: { name: web }
spec:
  scaleTargetRef: { apiVersion: apps/v1, kind: Deployment, name: web }
  minReplicas: 1
  maxReplicas: 5
  metrics:
  - type: Resource
    resource: { name: cpu, target: { type: Utilization, averageUtilization: 50 } }
```

- [ ] **Step 5: base/kustomization.yaml**

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - api-v1.yaml
  - api-v2.yaml
  - web.yaml
  - hpa.yaml
```

- [ ] **Step 6: kubeconform で静的検証**

```bash
cd 10_container/02_kind_mesh
kubectl kustomize manifests/base | kubeconform -strict -ignore-missing-schemas -
```

Expected: error 0。

- [ ] **Step 7: Commit**

```bash
git add 10_container/02_kind_mesh/manifests/base
git commit -m "feat(10-2): manifests base — api-v1/v2 + web + service + HPA"
```

---

## Task 6: overlay istio-sidecar

**Files:**
- Create: `10_container/02_kind_mesh/manifests/overlays/istio-sidecar/{kustomization.yaml,namespace.yaml,virtualservice.yaml,destinationrule.yaml,peerauth.yaml,gateway.yaml}`

**Interfaces:**
- Consumes: `manifests/base/`
- Produces: namespace `istio-sidecar-app` 配下に base が複製、Istio injection 有効、`api-v1:api-v2 = 90:10` の VirtualService

- [ ] **Step 1: Istio 導入 (ambient profile、sidecar も同居可能)**

`Makefile` から呼ぶが、開発時は手で:

```bash
istioctl install --set profile=ambient -y
kubectl label namespace istio-sidecar-app istio-injection=enabled --overwrite
```

- [ ] **Step 2: overlays/istio-sidecar/namespace.yaml**

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: istio-sidecar-app
  labels: { istio-injection: enabled }
```

- [ ] **Step 3: virtualservice.yaml + destinationrule.yaml**

```yaml
apiVersion: networking.istio.io/v1
kind: DestinationRule
metadata: { name: api }
spec:
  host: api
  subsets:
  - name: v1
    labels: { version: v1 }
  - name: v2
    labels: { version: v2 }
---
apiVersion: networking.istio.io/v1
kind: VirtualService
metadata: { name: api }
spec:
  hosts: ["api"]
  http:
  - route:
    - destination: { host: api, subset: v1 }
      weight: 90
    - destination: { host: api, subset: v2 }
      weight: 10
```

- [ ] **Step 4: peerauth.yaml (mTLS strict)**

```yaml
apiVersion: security.istio.io/v1
kind: PeerAuthentication
metadata: { name: default }
spec: { mtls: { mode: STRICT } }
```

- [ ] **Step 5: gateway.yaml (Istio Gateway + VirtualService) で外部公開**

```yaml
apiVersion: networking.istio.io/v1
kind: Gateway
metadata: { name: api-gw }
spec:
  selector: { istio: ingressgateway }
  servers:
  - port: { number: 80, name: http, protocol: HTTP }
    hosts: ["sidecar.example"]
---
apiVersion: networking.istio.io/v1
kind: VirtualService
metadata: { name: api-ext }
spec:
  hosts: ["sidecar.example"]
  gateways: ["api-gw"]
  http:
  - route:
    - destination: { host: api, subset: v1 }
      weight: 90
    - destination: { host: api, subset: v2 }
      weight: 10
```

- [ ] **Step 6: kustomization.yaml**

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: istio-sidecar-app
resources:
  - namespace.yaml
  - ../../base
  - virtualservice.yaml
  - destinationrule.yaml
  - peerauth.yaml
  - gateway.yaml
```

- [ ] **Step 7: 投入 + canary 検証**

```bash
kubectl apply -k manifests/overlays/istio-sidecar
kubectl -n istio-sidecar-app wait --for=condition=Available deploy --all --timeout=120s
for i in $(seq 1 100); do curl -fsS http://localhost/api/v1/echo -H 'host: sidecar.example'; done | jq -r .version | sort | uniq -c
```

Expected: v1 が 85-95 件、v2 が 5-15 件。

- [ ] **Step 8: Commit**

```bash
git add 10_container/02_kind_mesh/manifests/overlays/istio-sidecar
git commit -m "feat(10-2): istio-sidecar overlay (canary 90:10 + mTLS strict + Gateway)"
```

---

## Task 7: overlay istio-ambient

**Files:**
- Create: `10_container/02_kind_mesh/manifests/overlays/istio-ambient/{kustomization.yaml,namespace.yaml,waypoint.yaml,authpolicy.yaml}`

**Interfaces:**
- Consumes: `manifests/base/`
- Produces: namespace `istio-ambient-app` を ambient データプレーン (ztunnel) 配下に置き、waypoint Gateway 経由で L7 AuthorizationPolicy を適用

- [ ] **Step 1: namespace.yaml**

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: istio-ambient-app
  labels: { istio.io/dataplane-mode: ambient }
```

- [ ] **Step 2: waypoint.yaml (Gateway API gateway をサービス用 waypoint として宣言)**

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: waypoint
  annotations:
    istio.io/for-service-account: default
spec:
  gatewayClassName: istio-waypoint
  listeners:
  - name: mesh
    port: 15008
    protocol: HBONE
```

- [ ] **Step 3: authpolicy.yaml (`/admin/*` を deny)**

```yaml
apiVersion: security.istio.io/v1
kind: AuthorizationPolicy
metadata: { name: deny-admin, namespace: istio-ambient-app }
spec:
  targetRefs: [{ kind: Service, group: "", name: api }]
  action: DENY
  rules:
  - to: [{ operation: { paths: ["/admin/*"] } }]
```

- [ ] **Step 4: kustomization.yaml**

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: istio-ambient-app
resources:
  - namespace.yaml
  - ../../base
  - waypoint.yaml
  - authpolicy.yaml
```

- [ ] **Step 5: 投入 + 検証**

```bash
kubectl apply -k manifests/overlays/istio-ambient
kubectl -n istio-ambient-app wait --for=condition=Available deploy --all --timeout=120s
kubectl -n istio-ambient-app exec deploy/web -- sh -c 'curl -s -o /dev/null -w "echo=%{http_code}\n" http://api/api/v1/echo; curl -s -o /dev/null -w "admin=%{http_code}\n" http://api/admin/secret'
```

Expected: `echo=200` / `admin=403`。

- [ ] **Step 6: Commit**

```bash
git add 10_container/02_kind_mesh/manifests/overlays/istio-ambient
git commit -m "feat(10-2): istio-ambient overlay with waypoint + AuthorizationPolicy"
```

---

## Task 8: overlay envoy-gateway

**Files:**
- Create: `10_container/02_kind_mesh/manifests/overlays/envoy-gateway/{kustomization.yaml,namespace.yaml,gatewayclass.yaml,gateway.yaml,httproute.yaml}`

- [ ] **Step 1: Envoy Gateway 導入**

```bash
helm upgrade --install eg oci://docker.io/envoyproxy/gateway-helm \
  --version v1.2.0 -n envoy-gateway-system --create-namespace
kubectl wait -n envoy-gateway-system --for=condition=Available deploy --all --timeout=180s
```

- [ ] **Step 2: namespace.yaml**

```yaml
apiVersion: v1
kind: Namespace
metadata: { name: envoy-gw-app }
```

- [ ] **Step 3: gatewayclass.yaml**

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: GatewayClass
metadata: { name: envoy }
spec: { controllerName: gateway.envoyproxy.io/gatewayclass-controller }
```

- [ ] **Step 4: gateway.yaml**

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata: { name: api-gw, namespace: envoy-gw-app }
spec:
  gatewayClassName: envoy
  listeners:
  - name: http
    port: 80
    protocol: HTTP
    hostname: gw.example
```

- [ ] **Step 5: httproute.yaml**

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata: { name: api, namespace: envoy-gw-app }
spec:
  parentRefs: [{ name: api-gw }]
  hostnames: ["gw.example"]
  rules:
  - matches: [{ path: { type: PathPrefix, value: /api } }]
    backendRefs: [{ name: api, port: 80 }]
```

- [ ] **Step 6: kustomization.yaml**

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: envoy-gw-app
resources:
  - namespace.yaml
  - ../../base
  - gatewayclass.yaml
  - gateway.yaml
  - httproute.yaml
```

- [ ] **Step 7: 検証**

```bash
kubectl apply -k manifests/overlays/envoy-gateway
sleep 10
curl -fsS http://localhost/api/v1/echo -H 'host: gw.example'
```

Expected: `{"version":"v1",...}`。

- [ ] **Step 8: Commit**

```bash
git add 10_container/02_kind_mesh/manifests/overlays/envoy-gateway
git commit -m "feat(10-2): envoy-gateway overlay (Gateway API GatewayClass/Gateway/HTTPRoute)"
```

---

## Task 9: overlay linkerd (別 cluster)

**Files:**
- Create: `10_container/02_kind_mesh/manifests/overlays/linkerd/{kustomization.yaml,namespace.yaml,serviceprofile.yaml,trafficsplit.yaml}`

- [ ] **Step 1: cluster 起動 + Linkerd 導入**

```bash
kind create cluster --config kind/cluster-linkerd.yaml
kubectl config use-context kind-learning-linkerd
linkerd install --crds | kubectl apply -f -
linkerd install | kubectl apply -f -
linkerd check
```

- [ ] **Step 2: namespace.yaml**

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: linkerd-app
  annotations: { linkerd.io/inject: enabled }
```

- [ ] **Step 3: serviceprofile.yaml**

```yaml
apiVersion: linkerd.io/v1alpha2
kind: ServiceProfile
metadata: { name: api.linkerd-app.svc.cluster.local, namespace: linkerd-app }
spec:
  routes:
  - name: echo
    condition: { method: GET, pathRegex: "/api/v[12]/echo" }
    timeout: 2s
    retryBudget:
      retryRatio: 0.2
      minRetriesPerSecond: 10
      ttl: 10s
```

- [ ] **Step 4: trafficsplit.yaml (SMI)**

```yaml
apiVersion: split.smi-spec.io/v1alpha2
kind: TrafficSplit
metadata: { name: api, namespace: linkerd-app }
spec:
  service: api
  backends:
  - { service: api-v1, weight: 90 }
  - { service: api-v2, weight: 10 }
```

注: SMI を使う場合は別途 Service `api-v1` / `api-v2` を split 用に追加 (kustomization で `patches` か追加 yaml)。

- [ ] **Step 5: kustomization.yaml**

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: linkerd-app
resources:
  - namespace.yaml
  - ../../base
  - serviceprofile.yaml
  - trafficsplit.yaml
```

- [ ] **Step 6: image load + 投入**

```bash
kind load docker-image localhost:5001/demo-api:v1 --name learning-linkerd
kind load docker-image localhost:5001/demo-api:v2 --name learning-linkerd
kind load docker-image localhost:5001/demo-web:latest --name learning-linkerd
kubectl apply -k manifests/overlays/linkerd
kubectl -n linkerd-app wait --for=condition=Available deploy --all --timeout=180s
linkerd -n linkerd-app stat deploy
```

Expected: web/api Deployment Ready、`linkerd stat` で proxy 経由 traffic。

- [ ] **Step 7: Commit**

```bash
git add 10_container/02_kind_mesh/manifests/overlays/linkerd
git commit -m "feat(10-2): linkerd overlay (ServiceProfile + SMI TrafficSplit) on separate cluster"
```

---

## Task 10: overlay cilium (別 cluster + L7 NetworkPolicy)

**Files:**
- Create: `10_container/02_kind_mesh/manifests/overlays/cilium/{kustomization.yaml,namespace.yaml,l7policy.yaml}`

- [ ] **Step 1: cluster + Cilium 導入**

```bash
kind create cluster --config kind/cluster-cilium.yaml
kubectl config use-context kind-learning-cilium
cilium install --version 1.16 \
  --set kubeProxyReplacement=true \
  --set k8sServiceHost=$(docker inspect learning-cilium-control-plane -f '{{ .NetworkSettings.Networks.kind.IPAddress }}') \
  --set k8sServicePort=6443
cilium hubble enable --ui
cilium status --wait
```

- [ ] **Step 2: namespace.yaml**

```yaml
apiVersion: v1
kind: Namespace
metadata: { name: cilium-app }
```

- [ ] **Step 3: l7policy.yaml (Cilium L7 HTTP)**

```yaml
apiVersion: cilium.io/v2
kind: CiliumNetworkPolicy
metadata: { name: api-l7, namespace: cilium-app }
spec:
  endpointSelector: { matchLabels: { app: api } }
  ingress:
  - fromEndpoints: [{ matchLabels: { app: web } }]
    toPorts:
    - ports: [{ port: "8080", protocol: TCP }]
      rules:
        http:
        - method: GET
          path: "/api/v[12]/echo"
        - method: GET
          path: "/healthz"
```

注: `/admin/*` は許可リストに含めない → deny される。

- [ ] **Step 4: kustomization.yaml**

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: cilium-app
resources:
  - namespace.yaml
  - ../../base
  - l7policy.yaml
```

- [ ] **Step 5: image load + 投入 + 検証**

```bash
kind load docker-image localhost:5001/demo-api:v1     --name learning-cilium
kind load docker-image localhost:5001/demo-api:v2     --name learning-cilium
kind load docker-image localhost:5001/demo-web:latest --name learning-cilium
kubectl apply -k manifests/overlays/cilium
kubectl -n cilium-app wait --for=condition=Available deploy --all --timeout=180s
kubectl -n cilium-app exec deploy/web -- sh -c 'curl -s -o /dev/null -w "echo=%{http_code}\n" http://api/api/v1/echo; curl -s -o /dev/null -w "admin=%{http_code}\n" http://api/admin/secret'
```

Expected: `echo=200` / `admin=403`。

- [ ] **Step 6: Commit**

```bash
git add 10_container/02_kind_mesh/manifests/overlays/cilium
git commit -m "feat(10-2): cilium overlay with L7 NetworkPolicy denying /admin/*"
```

---

## Task 11: overlay spinkube (WASM workload)

**Files:**
- Create: `10_container/02_kind_mesh/manifests/overlays/spinkube/{kustomization.yaml,namespace.yaml,runtimeclass.yaml,spinapp.yaml}`

- [ ] **Step 1: SpinKube 導入 (base cluster)**

```bash
kubectl config use-context kind-learning-base
# cert-manager (前提)
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.15.0/cert-manager.yaml
kubectl -n cert-manager wait --for=condition=Available deploy --all --timeout=180s
# Spin Operator + RuntimeClass
kubectl apply -f https://github.com/spinkube/spin-operator/releases/latest/download/spin-operator.runtime-class.yaml
kubectl apply -f https://github.com/spinkube/spin-operator/releases/latest/download/spin-operator.crds.yaml
helm upgrade --install spin-operator --namespace spin-operator --create-namespace \
  --version 0.4.0 oci://ghcr.io/spinkube/charts/spin-operator
```

- [ ] **Step 2: namespace.yaml**

```yaml
apiVersion: v1
kind: Namespace
metadata: { name: spinkube-app }
```

- [ ] **Step 3: runtimeclass.yaml (確認のため明示宣言、上で導入済なら no-op)**

(Step 1 で導入したため省略可、コメントで「Step 1 のマニフェストに含まれる」と明示)

- [ ] **Step 4: spinapp.yaml**

```yaml
apiVersion: core.spinkube.dev/v1alpha1
kind: SpinApp
metadata: { name: demo-spin, namespace: spinkube-app }
spec:
  image: "localhost:5001/demo-spin:v1"
  replicas: 2
  executor: containerd-shim-spin
```

- [ ] **Step 5: kustomization.yaml**

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: spinkube-app
resources:
  - namespace.yaml
  - spinapp.yaml
```

- [ ] **Step 6: 投入 + cold-start 計測**

```bash
kubectl apply -k manifests/overlays/spinkube
kubectl -n spinkube-app wait --for=condition=Ready pod -l core.spinkube.dev/app-name=demo-spin --timeout=120s
kubectl -n spinkube-app port-forward svc/demo-spin 8090:80 &
sleep 1
time curl -fsS http://localhost:8090/api/v1/echo
kill %1
```

Expected: `{"runtime":"spin","version":"v1-wasm"}`、初回応答が概ね数十 ms オーダ。

- [ ] **Step 7: Commit**

```bash
git add 10_container/02_kind_mesh/manifests/overlays/spinkube
git commit -m "feat(10-2): spinkube overlay running Spin (Rust→wasm) SpinApp"
```

---

## Task 12: overlay kata (RuntimeClass 比較)

**Files:**
- Create: `10_container/02_kind_mesh/manifests/overlays/kata/{kustomization.yaml,namespace.yaml,api-kata.yaml,api-standard.yaml}`

注: kind のデフォルトノードは Kata 直接サポート無いため、本章では「RuntimeClass による切替方法 + Pod manifest 例」を中心に扱い、実 Kata 実行は kind では skip するか kata-deploy の試験ノードを使う旨を docs に明記する。

- [ ] **Step 1: namespace.yaml**

```yaml
apiVersion: v1
kind: Namespace
metadata: { name: kata-app }
```

- [ ] **Step 2: api-standard.yaml**

```yaml
apiVersion: apps/v1
kind: Deployment
metadata: { name: api-standard, namespace: kata-app }
spec:
  replicas: 1
  selector: { matchLabels: { app: api, runtime: standard } }
  template:
    metadata: { labels: { app: api, runtime: standard } }
    spec:
      containers:
      - { name: api, image: localhost:5001/demo-api:v1, ports: [{ containerPort: 8080 }] }
```

- [ ] **Step 3: api-kata.yaml**

```yaml
apiVersion: apps/v1
kind: Deployment
metadata: { name: api-kata, namespace: kata-app }
spec:
  replicas: 1
  selector: { matchLabels: { app: api, runtime: kata } }
  template:
    metadata: { labels: { app: api, runtime: kata } }
    spec:
      runtimeClassName: kata           # kata-deploy で導入される RuntimeClass
      containers:
      - { name: api, image: localhost:5001/demo-api:v1, ports: [{ containerPort: 8080 }] }
```

- [ ] **Step 4: kustomization.yaml**

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: kata-app
resources:
  - namespace.yaml
  - api-standard.yaml
  - api-kata.yaml
```

- [ ] **Step 5: 投入 + RuntimeClass 比較**

```bash
kubectl apply -k manifests/overlays/kata
kubectl -n kata-app get deploy
kubectl -n kata-app exec deploy/api-standard -- cat /proc/1/cgroup | head -3
# api-kata は kind 上では Pending になる想定。RuntimeClass の宣言を確認することが目的。
kubectl -n kata-app get pods
```

Expected: `api-standard` Running、`api-kata` の Pod spec に `runtimeClassName: kata` が反映されていること (実 Kata ノードが無いため Running にならない場合がある、docs で明示)。

- [ ] **Step 6: Commit**

```bash
git add 10_container/02_kind_mesh/manifests/overlays/kata
git commit -m "feat(10-2): kata/gVisor overlay illustrating RuntimeClass selection"
```

---

## Task 13: overlay vcluster

**Files:**
- Create: `10_container/02_kind_mesh/manifests/overlays/vcluster/{values-vc-a.yaml,values-vc-b.yaml,README.md}`

注: vCluster は Helm install で完結するため Kustomize ではなく `helm upgrade` 形式で扱う。

- [ ] **Step 1: values-vc-a.yaml / values-vc-b.yaml**

```yaml
# values-vc-a.yaml
sync:
  toHost:
    ingresses: { enabled: false }
controlPlane:
  service: { spec: { type: ClusterIP } }
```

(b は a と同内容で別 namespace に install)

- [ ] **Step 2: README.md (overlay 用)**

```markdown
# vcluster overlay

Helm でデプロイ:
```sh
helm upgrade --install vc-a vcluster -n vc-a --create-namespace --repo https://charts.loft.sh -f values-vc-a.yaml
helm upgrade --install vc-b vcluster -n vc-b --create-namespace --repo https://charts.loft.sh -f values-vc-b.yaml
vcluster connect vc-a -n vc-a -- kubectl get ns
```

vc-a と vc-b は host cluster から見ると独立した namespace、内側は別 cluster。
```

- [ ] **Step 3: 投入 + 検証**

```bash
helm upgrade --install vc-a vcluster -n vc-a --create-namespace --repo https://charts.loft.sh -f manifests/overlays/vcluster/values-vc-a.yaml
helm upgrade --install vc-b vcluster -n vc-b --create-namespace --repo https://charts.loft.sh -f manifests/overlays/vcluster/values-vc-b.yaml
kubectl -n vc-a wait --for=condition=Ready pod -l app=vcluster --timeout=180s
vcluster connect vc-a -n vc-a -- kubectl get ns
```

Expected: vc-a の内側に default/kube-system 等が見える。

- [ ] **Step 4: Commit**

```bash
git add 10_container/02_kind_mesh/manifests/overlays/vcluster
git commit -m "feat(10-2): vcluster overlay (2 instances) for tenant isolation demo"
```

---

## Task 14: overlay karpenter (kwok provider)

**Files:**
- Create: `10_container/02_kind_mesh/manifests/overlays/karpenter/{kustomization.yaml,nodepool.yaml,load.yaml}`

- [ ] **Step 1: Karpenter (kwok provider) 導入**

```bash
kubectl apply -f https://github.com/awslabs/karpenter-provider-kwok/releases/latest/download/install.yaml
kubectl -n karpenter wait --for=condition=Available deploy --all --timeout=180s
```

- [ ] **Step 2: nodepool.yaml**

```yaml
apiVersion: karpenter.sh/v1
kind: NodePool
metadata: { name: kwok-default }
spec:
  template:
    metadata: { labels: { provisioner: kwok } }
    spec:
      requirements:
      - { key: kubernetes.io/os, operator: In, values: [linux] }
      - { key: karpenter.sh/capacity-type, operator: In, values: [on-demand] }
      nodeClassRef:
        group: karpenter.kwok.sh
        kind: KWOKNodeClass
        name: default
  limits: { cpu: "100" }
  disruption:
    consolidationPolicy: WhenEmpty
    consolidateAfter: 30s
---
apiVersion: karpenter.kwok.sh/v1alpha1
kind: KWOKNodeClass
metadata: { name: default }
spec: {}
```

- [ ] **Step 3: load.yaml (Deployment で意図的に多数 Pod 要求)**

```yaml
apiVersion: apps/v1
kind: Deployment
metadata: { name: karpenter-load, namespace: karpenter-demo }
spec:
  replicas: 20
  selector: { matchLabels: { app: load } }
  template:
    metadata: { labels: { app: load } }
    spec:
      containers:
      - name: pause
        image: registry.k8s.io/pause:3.10
        resources:
          requests: { cpu: 500m, memory: 64Mi }
```

- [ ] **Step 4: namespace + kustomization.yaml**

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - nodepool.yaml
  - load.yaml
namespace: karpenter-demo
```

`namespace.yaml` を追加して `karpenter-demo` を宣言、`resources` に追加する。

- [ ] **Step 5: 投入 + 観測**

```bash
kubectl apply -k manifests/overlays/karpenter
kubectl get nodepool,nodes -w &
sleep 30
kill %1 || true
kubectl get nodes -l provisioner=kwok
```

Expected: kwok ノードが追加され、`karpenter-load` の Pod がスケジュールされる。

- [ ] **Step 6: Commit**

```bash
git add 10_container/02_kind_mesh/manifests/overlays/karpenter
git commit -m "feat(10-2): karpenter overlay with kwok provider for autoscale demo"
```

---

## Task 15: Root Makefile + VERIFICATION.md

**Files:**
- Create: `10_container/02_kind_mesh/Makefile`
- Create: `10_container/02_kind_mesh/VERIFICATION.md`

**Interfaces:**
- Consumes: Task 1-14 の成果物
- Produces: `make up` → `make install-*` → `make verify` → `make down` の運用フロー

- [ ] **Step 1: Makefile**

```makefile
.PHONY: up down build push install-istio install-envoy-gw install-linkerd install-cilium \
        install-spinkube install-kata install-vcluster install-karpenter \
        demo-canary-sidecar demo-ambient demo-gateway demo-linkerd demo-l7-policy \
        demo-wasm demo-kata demo-vcluster demo-karpenter verify

up:
	./kind/bootstrap.sh
	kind create cluster --config kind/cluster-linkerd.yaml || true
	kind create cluster --config kind/cluster-cilium.yaml  || true
	$(MAKE) build push

build:
	docker build --build-arg APP_VERSION=v1 -t localhost:5001/demo-api:v1 demo-app/api
	docker build --build-arg APP_VERSION=v2 -t localhost:5001/demo-api:v2 demo-app/api
	docker build -t localhost:5001/demo-web:latest demo-app/web

push:
	docker push localhost:5001/demo-api:v1
	docker push localhost:5001/demo-api:v2
	docker push localhost:5001/demo-web:latest

install-istio:
	kubectl config use-context kind-learning-base
	istioctl install --set profile=ambient -y
	kubectl apply -k manifests/overlays/istio-sidecar
	kubectl apply -k manifests/overlays/istio-ambient

install-envoy-gw:
	kubectl config use-context kind-learning-base
	helm upgrade --install eg oci://docker.io/envoyproxy/gateway-helm \
	  --version v1.2.0 -n envoy-gateway-system --create-namespace
	kubectl apply -k manifests/overlays/envoy-gateway

install-linkerd:
	kubectl config use-context kind-learning-linkerd
	kind load docker-image localhost:5001/demo-api:v1     --name learning-linkerd
	kind load docker-image localhost:5001/demo-api:v2     --name learning-linkerd
	kind load docker-image localhost:5001/demo-web:latest --name learning-linkerd
	linkerd install --crds | kubectl apply -f -
	linkerd install | kubectl apply -f -
	kubectl apply -k manifests/overlays/linkerd

install-cilium:
	kubectl config use-context kind-learning-cilium
	kind load docker-image localhost:5001/demo-api:v1     --name learning-cilium
	kind load docker-image localhost:5001/demo-api:v2     --name learning-cilium
	kind load docker-image localhost:5001/demo-web:latest --name learning-cilium
	cilium install --version 1.16
	cilium hubble enable --ui
	kubectl apply -k manifests/overlays/cilium

install-spinkube:
	kubectl config use-context kind-learning-base
	kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.15.0/cert-manager.yaml
	kubectl apply -f https://github.com/spinkube/spin-operator/releases/latest/download/spin-operator.runtime-class.yaml
	kubectl apply -f https://github.com/spinkube/spin-operator/releases/latest/download/spin-operator.crds.yaml
	helm upgrade --install spin-operator --namespace spin-operator --create-namespace \
	  --version 0.4.0 oci://ghcr.io/spinkube/charts/spin-operator
	kubectl apply -k manifests/overlays/spinkube

install-kata:
	kubectl config use-context kind-learning-base
	kubectl apply -k manifests/overlays/kata

install-vcluster:
	kubectl config use-context kind-learning-base
	helm upgrade --install vc-a vcluster -n vc-a --create-namespace --repo https://charts.loft.sh -f manifests/overlays/vcluster/values-vc-a.yaml
	helm upgrade --install vc-b vcluster -n vc-b --create-namespace --repo https://charts.loft.sh -f manifests/overlays/vcluster/values-vc-b.yaml

install-karpenter:
	kubectl config use-context kind-learning-base
	kubectl apply -f https://github.com/awslabs/karpenter-provider-kwok/releases/latest/download/install.yaml
	kubectl apply -k manifests/overlays/karpenter

demo-canary-sidecar:
	for i in $$(seq 1 100); do curl -fsS http://localhost/api/v1/echo -H 'host: sidecar.example'; done | jq -r .version | sort | uniq -c

demo-ambient:
	kubectl -n istio-ambient-app exec deploy/web -- sh -c 'curl -s -o /dev/null -w "echo=%{http_code}\nadmin=%{http_code}\n" http://api/api/v1/echo http://api/admin/secret'

demo-gateway:
	curl -fsS http://localhost/api/v1/echo -H 'host: gw.example'

demo-l7-policy:
	kubectl --context kind-learning-cilium -n cilium-app exec deploy/web -- sh -c \
	  'curl -s -o /dev/null -w "echo=%{http_code}\n" http://api/api/v1/echo; \
	   curl -s -o /dev/null -w "admin=%{http_code}\n" http://api/admin/secret'

demo-wasm:
	kubectl -n spinkube-app port-forward svc/demo-spin 8090:80 >/dev/null 2>&1 &
	sleep 2
	curl -fsS http://localhost:8090/api/v1/echo
	pkill -f "port-forward svc/demo-spin" || true

demo-kata:
	kubectl -n kata-app get pods -o wide

demo-vcluster:
	vcluster connect vc-a -n vc-a -- kubectl get ns

demo-karpenter:
	kubectl get nodepool,nodes -l provisioner=kwok

verify:
	kubectl --context kind-learning-base    wait --for=condition=Ready pods --all -A --timeout=300s || true
	kubectl --context kind-learning-linkerd wait --for=condition=Ready pods --all -A --timeout=300s || true
	kubectl --context kind-learning-cilium  wait --for=condition=Ready pods --all -A --timeout=300s || true
	$(MAKE) demo-canary-sidecar
	$(MAKE) demo-ambient
	$(MAKE) demo-gateway
	$(MAKE) demo-l7-policy
	$(MAKE) demo-wasm

down:
	kind delete cluster --name learning-base    || true
	kind delete cluster --name learning-linkerd || true
	kind delete cluster --name learning-cilium  || true
	docker rm -f kind-registry || true
```

- [ ] **Step 2: VERIFICATION.md**

```markdown
# 10-2 検証手順

## 前提
- docker, kind v0.24+, kubectl, helm, istioctl, linkerd, cilium CLI, vcluster CLI, jq, spin CLI

## 段取り
```sh
cd 10_container/02_kind_mesh
make up
make install-istio
make install-envoy-gw
make install-spinkube
make install-kata
make install-vcluster
make install-karpenter
make install-linkerd
make install-cilium
make verify
```

期待:
- `demo-canary-sidecar`: v1 が 85-95、v2 が 5-15
- `demo-ambient`: `echo=200 / admin=403`
- `demo-gateway`: 200
- `demo-l7-policy`: `echo=200 / admin=403`
- `demo-wasm`: `{"runtime":"spin","version":"v1-wasm"}`

後始末:
```sh
make down
```
```

- [ ] **Step 3: 通し検証**

```bash
cd 10_container/02_kind_mesh
make up
# overlay は手で順に install （長時間）
make verify
```

Expected: `make verify` exit 0 (各 demo の期待値を満たす)。

- [ ] **Step 4: Commit**

```bash
git add 10_container/02_kind_mesh/{Makefile,VERIFICATION.md}
git commit -m "feat(10-2): root Makefile orchestrating all overlays + VERIFICATION"
```

---

## Task 16: docs 本文書き起こし

**Files:**
- Modify: `10_container/02_kind_mesh/docs/*.md` (11 ファイル)

- [ ] **Step 1: 01-kind-basics.md**

内容: kind の仕組、`cluster.yaml`、`extraPortMappings`、local registry mirror、kindnet vs Cilium、multi-cluster 運用。

- [ ] **Step 2: 02-istio-sidecar.md**

内容: data plane = Envoy sidecar、control plane = istiod、injection webhook、PeerAuthentication mTLS、VirtualService weight、DestinationRule subset、Gateway 経由外部公開、Kiali 観測。

- [ ] **Step 3: 03-istio-ambient.md**

内容: ztunnel (L4 secure overlay、HBONE)、waypoint proxy (L7)、sidecar との比較 (上書き運用容易性、IPv6、Pod 改修不要)、AuthorizationPolicy 適用順序、sidecar overlay と同居する label rule。

- [ ] **Step 4: 04-envoy-gateway.md**

内容: Gateway API 仕様 (GatewayClass / Gateway / HTTPRoute / TCPRoute / TLSRoute / GRPCRoute)、Ingress からの移行、Envoy Gateway の実装位置、Istio Gateway との比較。

- [ ] **Step 5: 05-linkerd.md**

内容: linkerd2-proxy (Rust micro-proxy)、SMI ServiceProfile / TrafficSplit、retries / timeout、Istio との設計思想差 (シンプル + 軽量)、`linkerd viz` 観測。

- [ ] **Step 6: 06-cilium-mesh.md**

内容: eBPF data path、kube-proxy 代替、Hubble flow、CiliumNetworkPolicy L3/L4/L7、Cluster Mesh (multi-cluster)、Service Mesh モード概略。

- [ ] **Step 7: 07-spinkube-wasm.md**

内容: SpinApp CRD、containerd-shim-spin、wasmtime runtime、cold-start / footprint 計測、wasm-on-k8s 採用基準 (event-driven workload、polyglot)、KubeVirt との対比 (1 行)。

- [ ] **Step 8: 08-kata-gvisor.md**

内容: RuntimeClass、Kata (lightweight VM)、gVisor (user-space kernel)、multi-tenant 利用、kind で動かない理由と本物環境への持ち出し、性能 trade-off。

- [ ] **Step 9: 09-vcluster.md**

内容: syncer 構成、host namespace ↔ vcluster namespace、用途 (CI、teaching env、namespace 超え隔離)、Loft / kcp / Crossplane との比較。

- [ ] **Step 10: 10-karpenter.md**

内容: NodePool / NodeClass、kwok provider で kind 学習、AWS EC2 Provider への展開、ECS Capacity Provider との対応 (10-3 cross-link)。

- [ ] **Step 11: compare.md**

内容: mesh feature matrix (mTLS / L7 routing / observability / sidecar 要否 / footprint)、runtime trade-off (kata/gvisor/wasm)、autoscaler 対応表。

- [ ] **Step 12: Commit**

```bash
git add 10_container/02_kind_mesh/docs
git commit -m "docs(10-2): full guides for kind + mesh/runtime + comparison matrix"
```

---

## Self-Review Notes

- Spec の 11 技術項目を Task 6-14 で網羅、Task 16 で docs 11 本対応
- demo-app (Go/Node/Spin) は Task 2-4、base manifests は Task 5、横串 Makefile + verify は Task 15
- Linkerd / Cilium は別 cluster (Task 9, 10) で隔離、image は `kind load docker-image` で投入
- Kata は kind ノード上では Running しない可能性を Task 12/docs 08 で明記
- SpinKube / vCluster / Karpenter は base cluster 同居 (Task 11-14)
- すべての manifest が kubeconform 通過、Task 5 で base を確認
