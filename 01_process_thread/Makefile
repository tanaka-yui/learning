# ============================================================
# Process & Thread — Kubernetes Auto-Scaling with kind
# ============================================================

CLUSTER_NAME         := process-thread
KIND_CONFIG          := k8s/kind/cluster.yaml
NAMESPACE            := process-thread
INGRESS_NGINX_VERSION := controller-v1.12.0

# Auto-detect platform (ARM Mac = arm64, Intel = amd64)
ARCH := $(shell uname -m)
ifeq ($(ARCH),arm64)
  PLATFORM := linux/arm64
else
  PLATFORM := linux/amd64
endif

# Docker image definitions
IMAGES       := typescript go java php ruby rust-actix rust-axum python
IMAGE_PREFIX := pt

# Build contexts
BUILD_CTX_typescript := ./typescript
BUILD_CTX_go         := ./go
BUILD_CTX_java       := ./java
BUILD_CTX_php        := ./php
BUILD_CTX_ruby       := ./ruby
BUILD_CTX_rust-actix := ./rust/actix-web
BUILD_CTX_rust-axum  := ./rust/axum
BUILD_CTX_python     := ./python

# K6_TARGET → k8s app directory name (under k8s/base/apps/)
K8S_DIR_go         := go
K8S_DIR_ts         := typescript
K8S_DIR_java       := java
K8S_DIR_php        := php
K8S_DIR_ruby       := ruby
K8S_DIR_rust-actix := rust-actix
K8S_DIR_rust-axum  := rust-axum
K8S_DIR_python     := python

K8S_APP_DIR := $(K8S_DIR_$(K6_TARGET))

# K6_TARGET → Docker image name (IMAGES list name)
IMAGE_NAME_go         := go
IMAGE_NAME_ts         := typescript
IMAGE_NAME_java       := java
IMAGE_NAME_php        := php
IMAGE_NAME_ruby       := ruby
IMAGE_NAME_rust-actix := rust-actix
IMAGE_NAME_rust-axum  := rust-axum
IMAGE_NAME_python     := python

K6_IMAGE_NAME := $(IMAGE_NAME_$(K6_TARGET))

# k6 settings
K6_BASE_URL       ?= http://host.docker.internal:8080
K6_TARGET         ?= go
K6_TARGET_VUS     ?= 50
K6_TEST_PROFILE   ?= ramping
K6_MAX_VUS        ?= 200
K6_STEP_SIZE      ?= 5
K6_STEP_DURATION  ?= 30s
K6_PEAK_VUS       ?= 80
K6_REQUEST_TIMEOUT   ?= 30s
K6_TIMEOUT_THRESHOLD ?= 5000
K6_WARMUP_VUS        ?= 5
K6_WARMUP_DURATION   ?= 30s
K6_RAMP_UP_DURATION  ?= 1m
K6_PEAK_DURATION     ?= 3m
K6_SUSTAIN_DURATION  ?= 2m
K6_COOLDOWN_DURATION ?= 2m

# Per-language HEAVY_CALC_N defaults for k6 tests (~200ms/req target)
HEAVY_CALC_N_go         ?= 39
HEAVY_CALC_N_ts         ?= 37
HEAVY_CALC_N_java       ?= 39
HEAVY_CALC_N_php        ?= 30
HEAVY_CALC_N_ruby       ?= 30
HEAVY_CALC_N_rust-actix ?= 39
HEAVY_CALC_N_rust-axum  ?= 39
HEAVY_CALC_N_python     ?= 29

K6_HEAVY_CALC_N   ?= $(HEAVY_CALC_N_$(K6_TARGET))

# ============================================================
# Help
# ============================================================

.DEFAULT_GOAL := help

.PHONY: help
help:
	@echo "=== Process & Thread — Kubernetes Auto-Scaling ==="
	@echo ""
	@echo "--- Quick Start ---"
	@echo "  make up                             全セットアップ + デプロイ"
	@echo "  make up-single K6_TARGET=go         指定言語のみデプロイ (1 CPU 占有)"
	@echo "  make down                           クラスタ削除"
	@echo ""
	@echo "--- Build ---"
	@echo "  make build                          全言語のDockerイメージをビルド"
	@echo "  make build-go                       Go のみビルド (他の言語も同様)"
	@echo ""
	@echo "--- Cluster ---"
	@echo "  make cluster-create                 kind クラスタ作成"
	@echo "  make cluster-delete                 kind クラスタ削除"
	@echo "  make load-images                    イメージを kind にロード"
	@echo ""
	@echo "--- Deploy ---"
	@echo "  make deploy                         インフラ + アプリをデプロイ"
	@echo "  make deploy-infra                   metrics-server + ingress-nginx"
	@echo "  make deploy-apps                    アプリケーションのみ"
	@echo "  make redeploy                       イメージ再ビルド + 再ロード + 再デプロイ"
	@echo ""
	@echo "--- Load Test (Docker k6) ---"
	@echo "  make test K6_TARGET=go              負荷テスト (ramping)"
	@echo "  make test-find-threshold K6_TARGET=go  タイムアウト閾値探索"
	@echo "  make test-find-threshold-all          全言語の閾値探索を一括実行"
	@echo "  make test-find-threshold-exclusive    全言語を1つずつ占有テスト"
	@echo "  make test-autoscale K6_TARGET=go    HPA 動作確認テスト"
	@echo "  Dashboard: http://localhost:5665    テスト中リアルタイム表示"
	@echo "  Report:    k6/results/report.html   テスト後 HTML レポート"
	@echo ""
	@echo "--- Monitor ---"
	@echo "  make status                         全リソースの状態確認"
	@echo "  make watch-hpa                      HPA リアルタイム監視"
	@echo "  make watch-pods                     Pod リアルタイム監視"
	@echo "  make logs K6_TARGET=go              指定サービスのログ表示"
	@echo ""
	@echo "--- Variables ---"
	@echo "  K6_TARGET=go|ts|java|php|ruby|rust-actix|rust-axum|python"
	@echo "  K6_TARGET_VUS=50     K6_TEST_PROFILE=ramping|constant|stress|spike|soak"
	@echo "  K6_PEAK_VUS=80       K6_STEP_SIZE=5"
	@echo "  K6_HEAVY_CALC_N=39   言語別デフォルト値あり (Go/Rust/Java=39, TS=37, Ruby/PHP=30, Python=29)"
	@echo "  K6_REQUEST_TIMEOUT=30s  K6_TIMEOUT_THRESHOLD=5000"
	@echo "  K6_WARMUP_VUS=5      K6_WARMUP_DURATION=30s   K6_RAMP_UP_DURATION=1m"
	@echo "  K6_PEAK_DURATION=3m  K6_SUSTAIN_DURATION=2m   K6_COOLDOWN_DURATION=2m"

# ============================================================
# Prerequisites check
# ============================================================

.PHONY: check-docker
check-docker:
	@docker info > /dev/null 2>&1 || \
		(echo "Error: Docker is not running." && \
		 echo "Start Docker Desktop or Rancher Desktop (dockerd mode)." && \
		 exit 1)

.PHONY: check-kind
check-kind:
	@command -v kind > /dev/null 2>&1 || \
		(echo "Error: kind is not installed." && \
		 echo "Install: brew install kind" && \
		 exit 1)

.PHONY: check-kubectl
check-kubectl:
	@command -v kubectl > /dev/null 2>&1 || \
		(echo "Error: kubectl is not installed." && \
		 echo "Install: brew install kubectl" && \
		 exit 1)

# ============================================================
# Quick Start
# ============================================================

.PHONY: up
up: check-docker check-kind check-kubectl build cluster-create load-images deploy
	@echo ""
	@echo "=== Setup complete ==="
	@echo "Access endpoints via: http://localhost:8080/<language>/heavy"
	@echo "  http://localhost:8080/go/heavy"
	@echo "  http://localhost:8080/ts/heavy"
	@echo "  http://localhost:8080/java/heavy"
	@echo "  http://localhost:8080/php/heavy"
	@echo "  http://localhost:8080/ruby/heavy"
	@echo "  http://localhost:8080/rust-actix/heavy"
	@echo "  http://localhost:8080/rust-axum/heavy"
	@echo "  http://localhost:8080/python/heavy"
	@echo ""
	@echo "Next steps:"
	@echo "  make watch-hpa                      # Terminal 2: HPA 監視"
	@echo "  make test-autoscale K6_TARGET=go    # Terminal 1: 負荷テスト"

.PHONY: up-single
up-single: check-docker check-kind check-kubectl build-$(K6_IMAGE_NAME) cluster-create load-$(K6_IMAGE_NAME) deploy-infra wait-infra deploy-single-app wait-single-app
	@echo ""
	@echo "=== $(K6_TARGET) deployed with 1 CPU ==="
	@echo "  http://localhost:8080/$(K6_TARGET)/heavy"
	@echo ""
	@echo "Next steps:"
	@echo "  make test-find-threshold K6_TARGET=$(K6_TARGET)"

.PHONY: deploy-single-app
deploy-single-app:
	@echo "Deploying $(K6_TARGET) only..."
	kubectl apply -f k8s/base/namespace.yaml
	@echo "Cleaning up existing apps..."
	kubectl delete deployment -n $(NAMESPACE) --all --wait=true 2>/dev/null || true
	kubectl delete service -n $(NAMESPACE) -l app 2>/dev/null || true
	kubectl delete hpa -n $(NAMESPACE) --all 2>/dev/null || true
	@echo "Waiting for old pods to terminate..."
	@while kubectl get pods -n $(NAMESPACE) --no-headers 2>/dev/null | grep -q .; do \
		sleep 1; \
	done
	kubectl apply -f k8s/base/ingress.yaml
	kubectl apply -f k8s/base/apps/$(K8S_APP_DIR)/deployment.yaml
	kubectl apply -f k8s/base/apps/$(K8S_APP_DIR)/service.yaml
	kubectl apply -f k8s/base/apps/$(K8S_APP_DIR)/hpa.yaml

.PHONY: wait-single-app
wait-single-app:
	@echo "Waiting for $(K6_TARGET)-server pod..."
	kubectl wait --namespace $(NAMESPACE) \
		--for=condition=ready pod \
		--selector=app=$(K6_TARGET)-server \
		--timeout=180s
	@echo "Waiting for ingress health check..."
	@for i in $$(seq 1 30); do \
		if curl -sf http://localhost:8080/$(K6_TARGET)/health > /dev/null 2>&1; then \
			echo "  Health check passed."; \
			break; \
		fi; \
		if [ $$i -eq 30 ]; then \
			echo "  Warning: Health check not responding after 30s"; \
		fi; \
		sleep 1; \
	done

.PHONY: down
down: cluster-delete
	@echo "=== Cleanup complete ==="

# ============================================================
# Build
# ============================================================

.PHONY: build
build: check-docker $(addprefix build-,$(IMAGES))
	@echo "=== All images built ==="

define BUILD_RULE
.PHONY: build-$(1)
build-$(1):
	@echo "Building $(IMAGE_PREFIX)-$(1)..."
	docker build --platform $(PLATFORM) \
		-t $(IMAGE_PREFIX)-$(1):latest \
		$(BUILD_CTX_$(1))
endef

$(foreach img,$(IMAGES),$(eval $(call BUILD_RULE,$(img))))

# ============================================================
# Cluster
# ============================================================

.PHONY: cluster-create
cluster-create: check-kind
	@if kind get clusters 2>/dev/null | grep -q "^$(CLUSTER_NAME)$$"; then \
		echo "Cluster '$(CLUSTER_NAME)' already exists."; \
	else \
		kind create cluster --name $(CLUSTER_NAME) --config $(KIND_CONFIG); \
	fi
	kubectl cluster-info --context kind-$(CLUSTER_NAME)

.PHONY: cluster-delete
cluster-delete:
	@if kind get clusters 2>/dev/null | grep -q "^$(CLUSTER_NAME)$$"; then \
		kind delete cluster --name $(CLUSTER_NAME); \
	else \
		echo "Cluster '$(CLUSTER_NAME)' does not exist."; \
	fi

# ============================================================
# Load Images
# ============================================================

.PHONY: load-images
load-images: $(addprefix load-,$(IMAGES))
	@echo "=== All images loaded into kind ==="

define LOAD_RULE
.PHONY: load-$(1)
load-$(1):
	@echo "Loading $(IMAGE_PREFIX)-$(1) into kind..."
	kind load docker-image $(IMAGE_PREFIX)-$(1):latest --name $(CLUSTER_NAME)
endef

$(foreach img,$(IMAGES),$(eval $(call LOAD_RULE,$(img))))

# ============================================================
# Deploy
# ============================================================

.PHONY: deploy
deploy: deploy-infra wait-infra deploy-apps wait-apps
	@echo "=== All services deployed ==="

.PHONY: deploy-infra
deploy-infra:
	@echo "Deploying ingress-nginx controller ($(INGRESS_NGINX_VERSION))..."
	kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/$(INGRESS_NGINX_VERSION)/deploy/static/provider/kind/deploy.yaml
	@echo ""
	@echo "Deploying metrics-server (with --kubelet-insecure-tls for kind)..."
	@curl -sL https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml \
		| sed 's/args:/args:\n        - --kubelet-insecure-tls/' \
		| kubectl apply -f -

.PHONY: wait-infra
wait-infra:
	@echo "Waiting for ingress-nginx controller to be ready..."
	kubectl wait --namespace ingress-nginx \
		--for=condition=ready pod \
		--selector=app.kubernetes.io/component=controller \
		--timeout=180s
	@echo "Waiting for metrics-server to be ready..."
	kubectl wait --namespace kube-system \
		--for=condition=ready pod \
		--selector=k8s-app=metrics-server \
		--timeout=120s
	@echo "Infra is ready."

.PHONY: deploy-apps
deploy-apps:
	@echo "Deploying applications..."
	kubectl apply -k k8s/overlays/local/

.PHONY: wait-apps
wait-apps:
	@echo "Waiting for all pods to be ready..."
	@for app in go-server ts-server java-server php-server ruby-server rust-actix-server rust-axum-server python-server; do \
		echo "  Waiting for $$app..."; \
		kubectl wait --namespace $(NAMESPACE) \
			--for=condition=ready pod \
			--selector=app=$$app \
			--timeout=180s 2>/dev/null || \
			echo "  Warning: $$app not ready yet (may need more time)"; \
	done
	@echo "Apps deployment complete."

.PHONY: redeploy
redeploy: build load-images
	kubectl rollout restart deployment -n $(NAMESPACE)
	@echo "Waiting for rollout to complete..."
	@for app in go-server ts-server java-server php-server ruby-server rust-actix-server rust-axum-server python-server; do \
		kubectl rollout status deployment/$$app -n $(NAMESPACE) --timeout=180s 2>/dev/null || true; \
	done

# ============================================================
# Load Test (via Docker k6)
# ============================================================

.PHONY: test
test:
	docker run --rm \
		--add-host=host.docker.internal:host-gateway \
		-p 5665:5665 \
		-v $(CURDIR)/k6:/scripts \
		-w /scripts \
		-e K6_WEB_DASHBOARD=true \
		-e K6_WEB_DASHBOARD_EXPORT=/scripts/results/report.html \
		-e BASE_URL=$(K6_BASE_URL)/$(K6_TARGET) \
		-e ENDPOINT=/heavy \
		-e TEST_PROFILE=$(K6_TEST_PROFILE) \
		-e TARGET_VUS=$(K6_TARGET_VUS) \
		-e MAX_VUS=$(K6_MAX_VUS) \
		-e HEAVY_CALC_N=$(K6_HEAVY_CALC_N) \
		grafana/k6 run /scripts/heavy-load.js

.PHONY: test-find-threshold
test-find-threshold:
	docker run --rm \
		--add-host=host.docker.internal:host-gateway \
		-p 5665:5665 \
		-v $(CURDIR)/k6:/scripts \
		-w /scripts \
		-e K6_WEB_DASHBOARD=true \
		-e K6_WEB_DASHBOARD_EXPORT=/scripts/results/report.html \
		-e BASE_URL=$(K6_BASE_URL)/$(K6_TARGET) \
		-e ENDPOINT=/heavy \
		-e MAX_VUS=$(K6_MAX_VUS) \
		-e STEP_SIZE=$(K6_STEP_SIZE) \
		-e STEP_DURATION=$(K6_STEP_DURATION) \
		-e HEAVY_CALC_N=$(K6_HEAVY_CALC_N) \
		grafana/k6 run /scripts/threshold-finder.js

K6_TARGETS := go ts java php ruby rust-actix rust-axum python

.PHONY: test-find-threshold-all
test-find-threshold-all:
	@echo "=== Threshold Finder: All Languages ==="
	@for target in $(K6_TARGETS); do \
		echo ""; \
		echo "========================================"; \
		echo "  Testing: $$target"; \
		echo "========================================"; \
		$(MAKE) test-find-threshold K6_TARGET=$$target || true; \
	done
	@echo ""
	@echo "=== All languages tested ==="

.PHONY: _build-and-load-single
_build-and-load-single:
	$(MAKE) build-$(K6_IMAGE_NAME)
	$(MAKE) load-$(K6_IMAGE_NAME)

.PHONY: test-find-threshold-exclusive
test-find-threshold-exclusive: check-docker check-kind check-kubectl cluster-create deploy-infra wait-infra
	@echo "=== Exclusive Threshold Finder: All Languages (1 CPU each) ==="
	@for target in $(K6_TARGETS); do \
		echo ""; \
		echo "========================================"; \
		echo "  Testing: $$target (exclusive, 1 CPU)"; \
		echo "========================================"; \
		$(MAKE) _build-and-load-single K6_TARGET=$$target; \
		$(MAKE) deploy-single-app K6_TARGET=$$target; \
		$(MAKE) wait-single-app K6_TARGET=$$target; \
		$(MAKE) test-find-threshold K6_TARGET=$$target || true; \
	done
	@echo ""
	@echo "=== All languages tested (exclusive, 1 CPU) ==="

.PHONY: test-autoscale
test-autoscale:
	docker run --rm \
		--add-host=host.docker.internal:host-gateway \
		-p 5665:5665 \
		-v $(CURDIR)/k6:/scripts \
		-w /scripts \
		-e K6_WEB_DASHBOARD=true \
		-e K6_WEB_DASHBOARD_EXPORT=/scripts/results/report.html \
		-e BASE_URL=$(K6_BASE_URL)/$(K6_TARGET) \
		-e ENDPOINT=/heavy \
		-e PEAK_VUS=$(K6_PEAK_VUS) \
		-e HEAVY_CALC_N=$(K6_HEAVY_CALC_N) \
		-e REQUEST_TIMEOUT=$(K6_REQUEST_TIMEOUT) \
		-e TIMEOUT_THRESHOLD=$(K6_TIMEOUT_THRESHOLD) \
		-e WARMUP_VUS=$(K6_WARMUP_VUS) \
		-e WARMUP_DURATION=$(K6_WARMUP_DURATION) \
		-e RAMP_UP_DURATION=$(K6_RAMP_UP_DURATION) \
		-e PEAK_DURATION=$(K6_PEAK_DURATION) \
		-e SUSTAIN_DURATION=$(K6_SUSTAIN_DURATION) \
		-e COOLDOWN_DURATION=$(K6_COOLDOWN_DURATION) \
		grafana/k6 run /scripts/k8s-autoscale-test.js

# ============================================================
# Monitoring
# ============================================================

.PHONY: status
status:
	@echo "=== Nodes ==="
	@kubectl get nodes -o wide 2>/dev/null || echo "Cluster not running"
	@echo ""
	@echo "=== HPAs ==="
	@kubectl get hpa -n $(NAMESPACE) 2>/dev/null || echo "No HPAs found"
	@echo ""
	@echo "=== Pods ==="
	@kubectl get pods -n $(NAMESPACE) -o wide 2>/dev/null || echo "No pods found"
	@echo ""
	@echo "=== Services ==="
	@kubectl get svc -n $(NAMESPACE) 2>/dev/null || echo "No services found"
	@echo ""
	@echo "=== Ingress ==="
	@kubectl get ingress -n $(NAMESPACE) 2>/dev/null || echo "No ingress found"

.PHONY: watch-hpa
watch-hpa:
	kubectl get hpa -n $(NAMESPACE) -w

.PHONY: watch-pods
watch-pods:
	kubectl get pods -n $(NAMESPACE) -w

.PHONY: logs
logs:
	kubectl logs -n $(NAMESPACE) -l app=$(K6_TARGET)-server --tail=50 -f

.PHONY: events
events:
	kubectl get events -n $(NAMESPACE) --sort-by='.lastTimestamp'
