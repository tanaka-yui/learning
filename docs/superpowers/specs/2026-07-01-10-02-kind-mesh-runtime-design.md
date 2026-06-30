# 10-2 kind + 最新コンテナエコシステム — 設計仕様

- 作成日: 2026-07-01
- 章: `10_container/02_kind_mesh/`
- 関連: `06_microservie` (gRPC + frontend)、`07_network` (Envoy/network)、`10-1` (Dockerfile pattern を流用)、`10-3` (cross-link)

## 目的

kind を中心に、現在主流の k8s 周辺技術 (service mesh / Gateway API / CNI / WASM workload / sandbox runtime / virtual cluster / autoscaler) を「動く manifests + docs」で横断的に学習する。

## スコープ

採用技術:
- **Istio (sidecar + ambient)** — sidecar / ambient (ztunnel + waypoint) 両方
- **Envoy Gateway** — Gateway API 実装 (GatewayClass / Gateway / HTTPRoute)
- **Linkerd** — Rust micro-proxy、軽量比較軸
- **Cilium** — CNI 兼 Service Mesh (eBPF, Hubble, L7 NetworkPolicy)
- **SpinKube** — WASM workload (SpinApp CRD)
- **Kata Containers / gVisor** — RuntimeClass、sandbox runtime
- **vCluster** — 仮想 k8s クラスタ
- **Karpenter (OSS)** — kwok provider 経由で kind 上に擬似 autoscale

スコープ外:
- 実 AWS/GCP/Azure クラスタへのデプロイ
- マネージド mesh (App Mesh, GKE Anthos Mesh)
- 商用 Karpenter Cloud Provider (AWS) — 概念のみ docs で触れる

## アーキテクチャ

```
10_container/02_kind_mesh/
├── docs/
│   ├── README.md
│   ├── 01-kind-basics.md
│   ├── 02-istio-sidecar.md
│   ├── 03-istio-ambient.md
│   ├── 04-envoy-gateway.md
│   ├── 05-linkerd.md
│   ├── 06-cilium-mesh.md
│   ├── 07-spinkube-wasm.md
│   ├── 08-kata-gvisor.md
│   ├── 09-vcluster.md
│   ├── 10-karpenter.md
│   └── compare.md
├── demo-app/
│   ├── api/                # Go: GET /api/v1/echo, GET /api/v2/echo (version 差で canary)
│   │   ├── main.go
│   │   ├── Dockerfile      # 10-1 の B パターン流用
│   │   └── go.mod
│   ├── web/                # Node Fastify: api を呼んでレンダリング
│   │   ├── server.js
│   │   ├── package.json
│   │   └── Dockerfile
│   └── spin/               # 同等 API を Spin (Rust → wasm) で再実装
│       ├── src/lib.rs
│       ├── Cargo.toml
│       └── spin.toml
├── kind/
│   ├── cluster.yaml             # base cluster (3 node, ingress hostPort, registry)
│   ├── cluster-linkerd.yaml
│   ├── cluster-cilium.yaml
│   └── bootstrap.sh             # local registry + image push helper
├── manifests/
│   ├── base/                    # Kustomize base: Deployment(api-v1, api-v2, web), Service, HPA
│   │   ├── api-v1.yaml
│   │   ├── api-v2.yaml
│   │   ├── web.yaml
│   │   ├── hpa.yaml
│   │   └── kustomization.yaml
│   └── overlays/
│       ├── istio-sidecar/       # injection label + VirtualService (90:10) + DestinationRule + PeerAuthentication
│       ├── istio-ambient/       # ambient ns label + waypoint + AuthorizationPolicy
│       ├── envoy-gateway/       # GatewayClass + Gateway + HTTPRoute + TLSRoute
│       ├── linkerd/             # linkerd.io/inject + ServiceProfile + TrafficSplit (SMI)
│       ├── cilium/              # CiliumNetworkPolicy L7 + Hubble UI
│       ├── spinkube/            # SpinApp CRD + RuntimeClass=wasmtime-spin
│       ├── kata/                # 同一 Deployment を runtimeClassName: kata で複製、gVisor 版も並置
│       ├── vcluster/            # vcluster Helm values + 2 instance
│       └── karpenter/           # NodePool + EC2NodeClass (kwok provider)
├── Makefile
├── README.md
└── VERIFICATION.md
```

## demo-app

`api`:
- `GET /api/v1/echo` → `{"version":"v1","host":"<pod>"}`
- `GET /api/v2/echo` → `{"version":"v2","host":"<pod>"}`
- `GET /healthz` → 200
- `GET /admin/secret` → 200 (Cilium L7 policy で deny されることを示す用)

`web`:
- `GET /` → api を呼んで version 分布表示 (canary 動作確認用)

`spin`:
- 同じ `/api/v1/echo` を Spin (wasm) で実装、cold-start / footprint 比較

## クラスタ構成

注: Istio は ambient profile でインストールすれば sidecar と ambient を同一 cluster 内に namespace 単位で同居できる (namespace label `istio.io/dataplane-mode=ambient` 有無で振り分け)。docs/02 と 03 でこの方針を明記する。

- **base** (`learning-base`): Istio (sidecar/ambient 同居)、Envoy Gateway、SpinKube、Kata、vCluster、Karpenter
- **linkerd** (`learning-linkerd`): Linkerd 専用 (control plane 競合回避)
- **cilium** (`learning-cilium`): kindnet → Cilium 差替 (CNI 競合回避)

Namespace 分離:
- `istio-sidecar-app`, `istio-ambient-app`, `envoy-gw-app`, `spinkube-app`, `kata-app`, `vcluster-a`, `vcluster-b`, `karpenter-demo`

## 各 overlay の学習要点

| Overlay | 学習対象 | デモコマンド |
|---|---|---|
| istio-sidecar | mTLS, VirtualService weight=90:10, DestinationRule subset | `make demo-canary-sidecar` |
| istio-ambient | ztunnel (L4 mTLS) + waypoint (L7 policy)、sidecar 不要 | `make demo-ambient` |
| envoy-gateway | Gateway API GatewayClass/Gateway/HTTPRoute/TLSRoute | `make demo-gateway` |
| linkerd | ServiceProfile, retries/timeout, TrafficSplit | `make demo-linkerd` |
| cilium | L7 HTTP NetworkPolicy で `/admin/secret` deny、Hubble flow | `make demo-l7-policy` |
| spinkube | SpinApp CRD で WASM 起動、cold-start 計測 | `make demo-wasm` |
| kata | RuntimeClass=kata と standard 比較、`/proc/1/cgroup` 差分 | `make demo-kata` |
| vcluster | host から見えない vcluster 内 namespace | `make demo-vcluster` |
| karpenter | 負荷 → NodePool スケール、kwok でノード追加可視化 | `make demo-karpenter` |

## Makefile (10-2 root)

```makefile
.PHONY: up down build push install-% demo-% verify

up:
	./kind/bootstrap.sh                          # base cluster + registry
	kind create cluster --name learning-linkerd --config kind/cluster-linkerd.yaml
	kind create cluster --name learning-cilium  --config kind/cluster-cilium.yaml
	$(MAKE) build push

build:
	docker build -t localhost:5001/demo-api:v1 demo-app/api --build-arg APP_VERSION=v1
	docker build -t localhost:5001/demo-api:v2 demo-app/api --build-arg APP_VERSION=v2
	docker build -t localhost:5001/demo-web:latest demo-app/web

push:
	docker push localhost:5001/demo-api:v1
	docker push localhost:5001/demo-api:v2
	docker push localhost:5001/demo-web:latest

install-istio:
	istioctl install --set profile=ambient -y
	kubectl apply -k manifests/overlays/istio-sidecar
	kubectl apply -k manifests/overlays/istio-ambient

install-envoy-gw:
	helm upgrade --install eg oci://docker.io/envoyproxy/gateway-helm --version v1.2.0 -n envoy-gateway-system --create-namespace
	kubectl apply -k manifests/overlays/envoy-gateway

install-linkerd:
	kubectl config use-context kind-learning-linkerd
	linkerd install --crds | kubectl apply -f -
	linkerd install | kubectl apply -f -
	kubectl apply -k manifests/overlays/linkerd

install-cilium:
	kubectl config use-context kind-learning-cilium
	cilium install --version 1.16
	cilium hubble enable --ui
	kubectl apply -k manifests/overlays/cilium

install-spinkube:
	kubectl apply -f https://github.com/spinkube/spin-operator/releases/latest/download/spin-operator.crds.yaml
	helm upgrade --install spin-operator --namespace spin-operator --create-namespace oci://ghcr.io/spinkube/charts/spin-operator
	kubectl apply -k manifests/overlays/spinkube

install-kata:
	kubectl apply -f https://raw.githubusercontent.com/kata-containers/kata-containers/main/tools/packaging/kata-deploy/kata-deploy/base/kata-deploy.yaml
	kubectl apply -k manifests/overlays/kata

install-vcluster:
	helm upgrade --install vc-a vcluster -n vc-a --create-namespace --repo https://charts.loft.sh
	helm upgrade --install vc-b vcluster -n vc-b --create-namespace --repo https://charts.loft.sh

install-karpenter:
	# kwok-provider で擬似 (kind に EC2 は無いため)
	kubectl apply -f https://github.com/awslabs/karpenter-provider-kwok/releases/latest/download/install.yaml
	kubectl apply -k manifests/overlays/karpenter

demo-canary-sidecar:
	for i in $$(seq 1 100); do curl -fsS http://localhost/api/v1/echo -H 'host: sidecar.example'; done | jq -r .version | sort | uniq -c

demo-ambient:        ; curl -fsS http://localhost/api/v1/echo -H 'host: ambient.example'
demo-gateway:        ; curl -fsS http://localhost/api/v1/echo -H 'host: gw.example'
demo-linkerd:        ; kubectl --context kind-learning-linkerd -n linkerd-app exec deploy/web -- curl -fsS http://api/api/v1/echo
demo-l7-policy:
	# 期待: /api/v1/echo 200, /admin/secret 403
	kubectl --context kind-learning-cilium -n cilium-app exec deploy/web -- sh -c 'curl -sS -o /dev/null -w "%{http_code}\n" http://api/api/v1/echo; curl -sS -o /dev/null -w "%{http_code}\n" http://api/admin/secret'
demo-wasm:
	curl -fsS http://localhost/spin/api/v1/echo
demo-kata:
	kubectl -n kata-app exec deploy/api-kata     -- cat /proc/1/cgroup
	kubectl -n kata-app exec deploy/api-standard -- cat /proc/1/cgroup
demo-vcluster:
	vcluster connect vc-a -n vc-a -- kubectl get ns
demo-karpenter:
	kubectl apply -f manifests/overlays/karpenter/load.yaml
	kubectl get nodepool,nodes -w

verify:
	kubectl wait --for=condition=Ready pods --all -A --timeout=300s --context kind-learning-base
	$(MAKE) demo-canary-sidecar
	$(MAKE) demo-l7-policy
	$(MAKE) demo-wasm

down:
	kind delete cluster --name learning-base    || true
	kind delete cluster --name learning-linkerd || true
	kind delete cluster --name learning-cilium  || true
```

## docs/ 構成

- `01-kind-basics.md`: kind cluster.yaml、local registry、ingress port mapping
- `02-istio-sidecar.md`: data plane = Envoy sidecar、control plane = istiod、PeerAuthentication mTLS、canary 設計
- `03-istio-ambient.md`: ztunnel (L4 secure overlay) + waypoint (L7) の分離、sidecar との trade-off
- `04-envoy-gateway.md`: Gateway API 仕様、Ingress からの移行、HTTPRoute / TLSRoute / TCPRoute
- `05-linkerd.md`: linkerd2-proxy (Rust)、SMI、Istio との設計思想差
- `06-cilium-mesh.md`: eBPF data path、Hubble 観測、CiliumNetworkPolicy L3/L4/L7
- `07-spinkube-wasm.md`: SpinApp CRD、wasmtime runtime、cold-start / footprint 計測
- `08-kata-gvisor.md`: RuntimeClass、Kata (lightweight VM) と gVisor (user-space kernel) の方式差
- `09-vcluster.md`: virtual cluster の syncer 構成、用途 (multi-tenant 学習環境、CI)
- `10-karpenter.md`: OSS Karpenter、NodePool/NodeClass、kwok provider で kind 上学習、ECS Capacity Provider との対応 (10-3 cross-link)
- `compare.md`: 横断比較表 (mesh feature matrix、runtime trade-off、autoscaler 対応)

## 検証

- `make verify` exit 0
- `make demo-canary-sidecar`: 100 req 中 v2 が 8-12 件
- `make demo-l7-policy`: `/api/v1/echo`→200、`/admin/secret`→403
- `make demo-wasm`: 200 応答
- VERIFICATION.md に全コマンド + 期待出力 + トラブルシュート

## テスト

- manifest 静的検証: `kubeconform -strict manifests/`
- istioctl analyze, linkerd check, cilium status
- demo シナリオを `make verify` 内に統合
