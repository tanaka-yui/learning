# 10-2 Kind Mesh — Verification Guide

## Table of Contents

1. [Prerequisites](#1-prerequisites)
2. [Full Sequence](#2-full-sequence)
3. [Expected Outputs by Demo](#3-expected-outputs-by-demo)
   - [demo-canary-sidecar](#demo-canary-sidecar)
   - [demo-ambient](#demo-ambient)
   - [demo-gateway](#demo-gateway)
   - [demo-linkerd](#demo-linkerd)
   - [demo-l7-policy](#demo-l7-policy)
   - [demo-wasm](#demo-wasm)
   - [demo-kata](#demo-kata)
   - [demo-vcluster](#demo-vcluster)
   - [demo-karpenter](#demo-karpenter)
4. [Known Limitations](#4-known-limitations)
5. [Teardown](#5-teardown)

---

## 1. Prerequisites

| Tool | Minimum version | Notes |
|------|----------------|-------|
| docker | 24+ | `docker info` to confirm daemon is running |
| kind | v0.24+ | `kind version` |
| kubectl | v1.30+ | `kubectl version --client` |
| helm | v3.15+ | `helm version` |
| istioctl | 1.23+ | `istioctl version` |
| linkerd CLI | edge or stable ≥ 2.14 | `linkerd version --client` |
| cilium CLI | v0.16+ | `cilium version` |
| vcluster CLI | v0.20+ | `vcluster version` |
| jq | 1.6+ | `jq --version` |
| spin CLI | v2+ | `spin --version` (only required to build/push the WASM image) |

Install guides:
- kind: <https://kind.sigs.k8s.io/docs/user/quick-start/#installation>
- istioctl: `curl -L https://istio.io/downloadIstio | sh -`
- linkerd: `curl --proto '=https' --tlsv1.2 -sSfL https://run.linkerd.io/install-edge | sh`
- cilium: <https://docs.cilium.io/en/stable/gettingstarted/k8s-install-default/#install-the-cilium-cli>
- vcluster: <https://www.vcluster.com/docs/vcluster/get-started/>

---

## 2. Full Sequence

```sh
cd 10_container/02_kind_mesh

# 1. Spin up three kind clusters + local registry + build & push images
make up

# 2. Install each overlay (order matters: istio before envoy-gw)
make install-istio       # base cluster: Istio ambient + sidecar overlays
make install-envoy-gw    # base cluster: Envoy Gateway HTTPRoute
make install-spinkube    # base cluster: SpinKube operator + SpinApp
make install-kata        # base cluster: kata RuntimeClass + demo pod
make install-vcluster    # base cluster: two vCluster instances (vc-a, vc-b)
make install-karpenter   # base cluster: no-op (kwok provider discontinued)
make install-linkerd     # linkerd cluster: Linkerd + SMI TrafficSplit
make install-cilium      # cilium cluster: Cilium CNI + L7 NetworkPolicy

# 3. Wait for all pods and run smoke tests
make verify
```

> **Why install-istio before install-envoy-gw?**
> `install-istio` applies Gateway API CRDs (`standard-install.yaml` v1.2.1).
> `install-envoy-gw` passes `--skip-crds` to avoid server-side apply conflicts
> with those same CRDs.

> **Why install-linkerd after the base cluster overlays?**
> Linkerd runs in a separate kind cluster (`kind-learning-linkerd`) and does not
> share state with the base cluster. Order with respect to base-cluster targets
> does not matter; it is listed last for clarity.

---

## 3. Expected Outputs by Demo

### demo-canary-sidecar

**Command:** `make demo-canary-sidecar`  
**Context:** `kind-learning-base`, namespace `istio-sidecar-app`  
**Mechanism:** Istio `VirtualService` splits traffic 90 % → api-v1, 10 % → api-v2.

```
 85   v1      ← roughly 85–95 out of 100
 15   v2      ← roughly 5–15 out of 100
```

Exact counts vary per run due to probabilistic load balancing. Any split in the
85/15 ± 10 range is acceptable.

---

### demo-ambient

**Command:** `make demo-ambient`  
**Context:** `kind-learning-base`, namespace `istio-ambient-app`  
**Mechanism:** Istio `AuthorizationPolicy` allows `GET /api/*` and denies `GET /admin/*`.

```
echo=200
admin=403
```

> **Waypoint label requirement:** the `istio-ambient-app` namespace must carry
> the annotation `istio.io/use-waypoint: waypoint` for L7 policies to be
> enforced. The `waypoint.yaml` in the overlay creates the Waypoint Gateway and
> the `authpolicy.yaml` references it. Verify with:
> `kubectl get namespace istio-ambient-app -o yaml | grep use-waypoint`

---

### demo-gateway

**Command:** `make demo-gateway`  
**Context:** `kind-learning-base`  
**Mechanism:** Envoy Gateway `HTTPRoute` routes `host: gw.example` to the demo API.

```json
{"host":"...","version":"v1"}
```

HTTP 200, JSON body from the Go API (`host` is the pod hostname, `version` is `APP_VERSION`).

---

### demo-linkerd

**Command:** `make demo-linkerd`  
**Context:** `kind-learning-linkerd`, namespace `linkerd-app`  
**Mechanism:** SMI `TrafficSplit` 90/10 between `api-v1` and `api-v2` services.

```
 88   v1      ← roughly 85–95 out of 100
 12   v2      ← roughly 5–15 out of 100
```

> **SMI CRD note:** Linkerd edge builds no longer bundle the SMI TrafficSplit
> CRD. `install-linkerd` applies it separately from
> `servicemeshinterface/smi-sdk-go`. If the CRD is missing, the TrafficSplit
> resource will fail to apply and all traffic will flow to the default `api`
> service (100 % one version).

---

### demo-l7-policy

**Command:** `make demo-l7-policy`  
**Context:** `kind-learning-cilium`, namespace `cilium-app`  
**Mechanism:** Cilium `CiliumNetworkPolicy` (L7 HTTP rules) allows `GET /api/*`
and denies `GET /admin/*`.

```
echo=200
admin=403
```

---

### demo-wasm

**Command:** `make demo-wasm`  
**Context:** `kind-learning-base`, namespace `spinkube-app`  
**Mechanism:** Port-forward to `svc/demo-spin`, then HTTP GET.

**If containerd-shim-spin is present (non-kind environment):**

```json
{"runtime":"spin","version":"v1-wasm"}
```

**If running on standard kind nodes (expected in this lab):**

The port-forward will fail because the SpinApp pod is `Pending`
(see [Known Limitations](#4-known-limitations)). The target uses `|| true` so
`make verify` does not abort.

---

### demo-kata

**Command:** `make demo-kata`  
**Context:** `kind-learning-base`, namespace `kata-app`

**If kata-qemu runtime is present (bare-metal / nested-virt environment):**

```
NAME             READY   STATUS    RESTARTS   AGE   NODE
api-kata-...     1/1     Running   0          ...   ...
```

**On standard kind nodes (expected in this lab):**

```
NAME             READY   STATUS    RESTARTS   AGE   NODE
api-kata-...     0/1     Pending   0          ...   <none>
```

Pod remains `Pending` because the `kata-qemu` RuntimeClass has no matching
runtime. This is expected — see [Known Limitations](#4-known-limitations).

---

### demo-vcluster

**Command:** `make demo-vcluster`  
**Context:** `kind-learning-base`

```
NAME              STATUS   AGE
default           Active   ...
kube-node-lease   Active   ...
kube-public       Active   ...
kube-system       Active   ...
```

Lists namespaces inside virtual cluster `vc-a`. A non-empty list confirms the
vCluster API server is reachable and functional.

---

### demo-karpenter

**Command:** `make demo-karpenter`

```
WARNING: karpenter-provider-kwok is discontinued. Skipping demo.
```

No-op. See [Known Limitations](#4-known-limitations).

---

## 4. Known Limitations

### SpinApp stays Pending (no containerd-shim-spin on kind nodes)

SpinKube requires `containerd-shim-spin` to be installed on each node so
containerd can invoke the Spin runtime. Standard kind node images do not include
this shim.

**Impact:** The `SpinApp` pod in `spinkube-app` remains `Pending`; `demo-wasm`
cannot reach the service.

**Workaround for real testing:** Use a VM-based cluster (k3s, RKE2, or a cloud
provider) and follow the SpinKube node setup guide:
<https://www.spinkube.dev/docs/install/node-setup/>

---

### Kata pods stay Pending (no kata-qemu runtime on kind nodes)

Kata Containers requires hardware virtualisation support and the `kata-qemu`
runtime to be installed on each node. kind nodes run inside Docker containers
and do not expose the necessary nested virtualisation capabilities.

**Impact:** The pod in `kata-app` requesting `runtimeClassName: kata-qemu`
remains `Pending`.

**Workaround for real testing:** Use a bare-metal node or a VM with nested
virtualisation enabled. Install Kata via the operator:
<https://github.com/kata-containers/kata-containers/tree/main/tools/packaging/kata-deploy>

---

### Karpenter kwok provider is discontinued (upstream archived)

The `karpenter-provider-kwok` GitHub repository
(`awslabs/karpenter-provider-kwok`) has been archived by AWS and is no longer
maintained. The install URL used in the plan no longer resolves to a working
release.

**Impact:** `make install-karpenter` is a no-op that prints a warning and exits
successfully. `make demo-karpenter` prints the same warning.

**Workaround:** To learn Karpenter node provisioning concepts, use a real cloud
provider (AWS `karpenter-provider-aws`) or a local kwok simulation built from
source. This is outside the scope of the current lab.

---

## 5. Teardown

```sh
make down
```

Deletes all three kind clusters (`learning-base`, `learning-linkerd`,
`learning-cilium`) and removes the local registry container (`kind-registry`).
Local Docker images pushed to `localhost:5001` remain in the registry volume
until the volume is pruned manually.
