.PHONY: up down build build-spin push \
        install-gateway-api-crds \
        install-istio install-envoy-gw install-linkerd install-cilium \
        install-spinkube install-kata install-vcluster install-karpenter \
        demo-canary-sidecar demo-ambient demo-gateway demo-linkerd demo-l7-policy \
        demo-wasm demo-kata demo-vcluster demo-karpenter verify

REGISTRY ?= localhost:5001

# ─── Cluster lifecycle ────────────────────────────────────────────────────────

up:
	./kind/bootstrap.sh
	kind create cluster --config kind/cluster-linkerd.yaml || true
	kind create cluster --config kind/cluster-cilium.yaml  || true
	$(MAKE) build push

down:
	kind delete cluster --name learning-base    || true
	kind delete cluster --name learning-linkerd || true
	kind delete cluster --name learning-cilium  || true
	docker rm -f kind-registry || true

# ─── Image build & push ───────────────────────────────────────────────────────

build: build-spin
	docker build --build-arg APP_VERSION=v1 -t $(REGISTRY)/demo-api:v1 demo-app/api
	docker build --build-arg APP_VERSION=v2 -t $(REGISTRY)/demo-api:v2 demo-app/api
	docker build -t $(REGISTRY)/demo-web:latest demo-app/web

build-spin:
	@if ! command -v spin >/dev/null 2>&1; then \
	  echo "WARNING: spin CLI not found — skipping Wasm image build"; \
	  exit 0; \
	else \
	  cd demo-app/spin && spin build && spin registry push $(REGISTRY)/demo-spin:v1 --insecure; \
	fi

push:
	docker push $(REGISTRY)/demo-api:v1
	docker push $(REGISTRY)/demo-api:v2
	docker push $(REGISTRY)/demo-web:latest

# ─── Overlay installs ─────────────────────────────────────────────────────────

# Gateway API CRDs are shared by Istio (install-istio) and Envoy Gateway
# (install-envoy-gw). Installing them once here prevents server-side apply
# conflicts. install-envoy-gw passes --skip-crds for the same reason.
install-gateway-api-crds:
	kubectl apply -f https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.2.1/standard-install.yaml
	kubectl wait --for=condition=Established crd/gateways.gateway.networking.k8s.io --timeout=60s

install-istio: install-gateway-api-crds
	kubectl config use-context kind-learning-base
	istioctl install --set profile=ambient -y
	kubectl apply -k manifests/overlays/istio-sidecar
	kubectl apply -k manifests/overlays/istio-ambient

install-envoy-gw: install-gateway-api-crds
	kubectl config use-context kind-learning-base
	# --skip-crds: Gateway API CRDs already applied by install-gateway-api-crds;
	# re-applying them causes server-side apply conflicts.
	helm upgrade --install eg oci://docker.io/envoyproxy/gateway-helm \
	  --version v1.2.0 -n envoy-gateway-system --create-namespace \
	  --skip-crds
	kubectl apply -k manifests/overlays/envoy-gateway

install-linkerd:
	kubectl config use-context kind-learning-linkerd
	kind load docker-image $(REGISTRY)/demo-api:v1     --name learning-linkerd
	kind load docker-image $(REGISTRY)/demo-api:v2     --name learning-linkerd
	kind load docker-image $(REGISTRY)/demo-web:latest --name learning-linkerd
	linkerd install --crds | kubectl apply -f -
	linkerd install | kubectl apply -f -
	linkerd check
	# SMI TrafficSplit CRD is no longer bundled with Linkerd edge builds.
	# Pinned to v0.5.0 (latest tagged release of smi-sdk-go) to avoid main-branch drift.
	kubectl apply -f https://raw.githubusercontent.com/servicemeshinterface/smi-sdk-go/v0.5.0/crds/split.yaml
	kubectl apply -k manifests/overlays/linkerd

install-cilium:
	kubectl config use-context kind-learning-cilium
	kind load docker-image $(REGISTRY)/demo-api:v1     --name learning-cilium
	kind load docker-image $(REGISTRY)/demo-api:v2     --name learning-cilium
	kind load docker-image $(REGISTRY)/demo-web:latest --name learning-cilium
	# No --version flag: use the CLI default (1.19+ as of 2026).
	# Pinning to 1.16 fails when the installed cilium CLI ships a newer default.
	cilium install
	cilium hubble enable --ui
	kubectl apply -k manifests/overlays/cilium

install-spinkube:
	kubectl config use-context kind-learning-base
	kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.15.0/cert-manager.yaml
	kubectl wait --for=condition=Available deployment/cert-manager-webhook \
	  -n cert-manager --timeout=120s
	# Pinned to v0.4.0 to match the helm chart version below.
	kubectl apply -f https://github.com/spinkube/spin-operator/releases/download/v0.4.0/spin-operator.runtime-class.yaml
	kubectl apply -f https://github.com/spinkube/spin-operator/releases/download/v0.4.0/spin-operator.crds.yaml
	helm upgrade --install spin-operator --namespace spin-operator --create-namespace \
	  --version 0.4.0 oci://ghcr.io/spinkube/charts/spin-operator
	kubectl apply -k manifests/overlays/spinkube
	# KNOWN LIMITATION: SpinApp pods remain Pending on kind because
	# containerd-shim-spin is not present on kind worker nodes.

install-kata:
	kubectl config use-context kind-learning-base
	kubectl apply -k manifests/overlays/kata
	# KNOWN LIMITATION: Kata pods remain Pending on kind because
	# the kata-qemu runtime class is not available on kind worker nodes.

install-vcluster:
	kubectl config use-context kind-learning-base
	helm upgrade --install vc-a vcluster -n vc-a --create-namespace \
	  --repo https://charts.loft.sh -f manifests/overlays/vcluster/values-vc-a.yaml
	helm upgrade --install vc-b vcluster -n vc-b --create-namespace \
	  --repo https://charts.loft.sh -f manifests/overlays/vcluster/values-vc-b.yaml

install-karpenter:
	@echo "WARNING: karpenter-provider-kwok upstream is discontinued (repository archived)."
	@echo "         install-karpenter is a no-op. See VERIFICATION.md Known Limitations."
	@exit 0

# ─── Demo targets ─────────────────────────────────────────────────────────────

# Expected: v1 ~90 requests, v2 ~10 requests
demo-canary-sidecar:
	for i in $$(seq 1 100); do curl -fsS http://localhost/api/v1/echo -H 'host: sidecar.example'; done \
	  | jq -r .version | sort | uniq -c

# Expected: echo=200, admin=403 (AuthorizationPolicy blocks /admin/*)
demo-ambient:
	kubectl -n istio-ambient-app exec deploy/web -- sh -c \
	  'curl -s -o /dev/null -w "echo=%{http_code}\n" http://api/api/v1/echo; \
	   curl -s -o /dev/null -w "admin=%{http_code}\n" http://api/admin/secret'

# Expected: HTTP 200 JSON response via Envoy Gateway HTTPRoute
demo-gateway:
	curl -fsS http://localhost/api/v1/echo -H 'host: gw.example'

# Expected: v1 ~90 requests, v2 ~10 requests (Linkerd TrafficSplit 90/10)
demo-linkerd:
	kubectl --context kind-learning-linkerd -n linkerd-app exec deploy/web -- \
	  sh -c 'for i in $$(seq 1 100); do curl -fsS http://api/api/v1/echo; done' \
	  | jq -r .version | sort | uniq -c

# Expected: echo=200, admin=403 (Cilium L7 NetworkPolicy blocks /admin/*)
demo-l7-policy:
	kubectl --context kind-learning-cilium -n cilium-app exec deploy/web -- sh -c \
	  'curl -s -o /dev/null -w "echo=%{http_code}\n" http://api/api/v1/echo; \
	   curl -s -o /dev/null -w "admin=%{http_code}\n" http://api/admin/secret'

# Expected: {"runtime":"spin","version":"v1-wasm"} (if shim is present; else pod Pending)
demo-wasm:
	kubectl -n spinkube-app port-forward svc/demo-spin 8090:80 >/dev/null 2>&1 &
	sleep 2
	curl -fsS http://localhost:8090/api/v1/echo || true
	pkill -f "port-forward svc/demo-spin" || true

# Expected: Pending (kata-qemu RuntimeClass not available on kind)
demo-kata:
	kubectl -n kata-app get pods -o wide

# Expected: list of namespaces inside virtual cluster vc-a
demo-vcluster:
	vcluster connect vc-a -n vc-a -- kubectl get ns

# No-op: karpenter-provider-kwok is discontinued
demo-karpenter:
	@echo "WARNING: karpenter-provider-kwok is discontinued. Skipping demo."

# ─── Verify ───────────────────────────────────────────────────────────────────

verify:
	kubectl --context kind-learning-base    wait --for=condition=Ready pods --all -A --timeout=300s || true
	kubectl --context kind-learning-linkerd wait --for=condition=Ready pods --all -A --timeout=300s || true
	kubectl --context kind-learning-cilium  wait --for=condition=Ready pods --all -A --timeout=300s || true
	$(MAKE) demo-canary-sidecar
	$(MAKE) demo-ambient
	$(MAKE) demo-gateway
	$(MAKE) demo-l7-policy
	$(MAKE) demo-wasm
