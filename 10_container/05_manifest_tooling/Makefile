.PHONY: kustomize-render-dev kustomize-render-stg kustomize-render-prod \
        kustomize-apply-dev kustomize-diff \
        install-servicemonitor-crd \
        helm-lint helm-template-dev helm-template-stg helm-template-prod \
        helm-install-dev helm-test helm-diff \
        compare verify down

CTX        ?= kind-learning-manifest
HELM       ?= helm
KUBECONFORM ?= kubeconform

# ---- Kustomize ----
kustomize-render-dev:  ; kubectl kustomize kustomize/overlays/dev
kustomize-render-stg:  ; kubectl kustomize kustomize/overlays/stg
kustomize-render-prod: ; kubectl kustomize kustomize/overlays/prod

kustomize-apply-dev:
	kubectl --context $(CTX) apply -k kustomize/overlays/dev
	kubectl --context $(CTX) -n demo-dev wait --for=condition=Available deploy --all --timeout=120s

install-servicemonitor-crd:
	kubectl --context $(CTX) apply --server-side \
	  -f https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/v0.79.2/example/prometheus-operator-crd/monitoring.coreos.com_servicemonitors.yaml

kustomize-diff:
	@echo "=== dev vs prod (kustomize) ==="
	diff <(kubectl kustomize kustomize/overlays/dev) <(kubectl kustomize kustomize/overlays/prod) || true

# ---- Helm ----
helm-lint:
	$(HELM) lint helm/charts/demo-api

helm-template-dev:  ; $(HELM) template demo-api helm/charts/demo-api -f helm/values-dev.yaml
helm-template-stg:  ; $(HELM) template demo-api helm/charts/demo-api -f helm/values-stg.yaml
helm-template-prod: ; $(HELM) template demo-api helm/charts/demo-api -f helm/values-prod.yaml

helm-install-dev:
	cd helm/charts/demo-api && $(HELM) dependency update
	kubectl --context $(CTX) create namespace demo-dev-helm 2>/dev/null || true
	$(HELM) --kube-context $(CTX) upgrade --install demo-api helm/charts/demo-api \
	  -f helm/values-dev.yaml -n demo-dev-helm
	kubectl --context $(CTX) -n demo-dev-helm wait --for=condition=Available deploy --all --timeout=120s

helm-test:
	$(HELM) --kube-context $(CTX) test demo-api -n demo-dev-helm

helm-diff:
	# helm-diff plugin 前提 (VERIFICATION に install 手順あり)
	$(HELM) --kube-context $(CTX) diff upgrade demo-api helm/charts/demo-api \
	  -f helm/values-prod.yaml -n demo-dev-helm || true

# ---- Compare ----
compare:
	@echo "=== kustomize prod resources ==="
	kubectl kustomize kustomize/overlays/prod | grep -E '^kind:|^  name:' | sort
	@echo ""
	@echo "=== helm prod resources ==="
	$(HELM) template demo-api helm/charts/demo-api -f helm/values-prod.yaml | grep -E '^kind:|^  name:' | sort
	@echo ""
	@echo "=== Both should render Deployment/Service/ConfigMap; helm has HPA + Ingress; kustomize has Ingress + ServiceMonitor. ==="

# ---- End-to-end ----
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
	$(MAKE) install-servicemonitor-crd
	$(MAKE) kustomize-apply-dev
	$(MAKE) helm-install-dev
	@echo "=== Helm test ==="
	$(MAKE) helm-test
	@echo "=== compare summary ==="
	$(MAKE) compare

down:
	-$(HELM) --kube-context $(CTX) uninstall demo-api -n demo-dev-helm
	-kubectl --context $(CTX) delete namespace demo-dev demo-dev-helm
	-kind delete cluster --name learning-manifest
