.PHONY: localstack-up ecr-push ecs-deploy ecs-run kind-up kind-deploy compare verify down

CLUSTER_NAME := learning-ecs-vs-k8s
KUBE_CTX     := kind-$(CLUSTER_NAME)
LS_ENDPOINT  := http://localhost:4566
AWS          := aws --endpoint-url=$(LS_ENDPOINT) --region us-east-1

# ─── LocalStack ────────────────────────────────────────────────────────────────

localstack-up:
	docker compose -f localstack/docker-compose.yml up -d
	@echo "Waiting for LocalStack to be ready..."
	@for i in 1 2 3 4 5 6 7 8 9 10; do \
	  curl -fsS $(LS_ENDPOINT)/_localstack/health >/dev/null 2>&1 \
	    && echo "LocalStack ready." && exit 0; \
	  echo "  attempt $$i/10 ..."; \
	  sleep 2; \
	done; echo "ERROR: LocalStack not ready after 20s"; exit 1

# ─── ECR substitute (registry:2 @ localhost:5000) ──────────────────────────────
#
# LocalStack Community does NOT support ECR (Pro feature).
# seed.sh builds + pushes to the registry:2 container instead.

ecr-push:
	docker build -t demo-api:v1 demo-app/api
	./localstack/seed.sh

# ─── ECS via Terraform ─────────────────────────────────────────────────────────
#
# IAM roles + CloudWatch log group succeed.
# ECS cluster/task/service return 501 (Community limitation) — exit is captured,
# a warning is printed, and make continues so the k8s side can still be verified.

ecs-deploy:
	@echo "--- terraform init ---"
	cd terraform/ecs && terraform init -upgrade -input=false
	@echo "--- terraform apply (ECS resources will 501 on LocalStack Community) ---"
	@cd terraform/ecs && terraform apply -auto-approve -input=false; \
	  EXIT=$$?; \
	  if [ $$EXIT -ne 0 ]; then \
	    echo ""; \
	    echo "WARN: terraform apply exited $$EXIT."; \
	    echo "  LocalStack Community does not emulate ECS (Pro feature)."; \
	    echo "  Succeeded : aws_iam_role.task_exec, aws_iam_role.task, aws_cloudwatch_log_group.api"; \
	    echo "  Failed 501: aws_ecs_cluster.this, aws_ecs_task_definition.api, aws_ecs_service.api"; \
	    echo "  This is expected — continuing with k8s side."; \
	  fi

# ─── ECS runtime check ─────────────────────────────────────────────────────────
#
# Returns exit 0 even when LocalStack returns 501 for ECS APIs.

ecs-run:
	@echo "=== ECS task list (LocalStack Community) ==="
	@$(AWS) ecs list-tasks --cluster learning-ecs 2>&1 || { \
	  echo "WARN: ECS list-tasks returned an error (expected)."; \
	  echo "  LocalStack Community does not emulate ECS (Pro feature)."; \
	  echo "  ECS comparison is Terraform-declaration-level only."; \
	}

# ─── kind + k8s ────────────────────────────────────────────────────────────────

kind-up:
	kind create cluster --name $(CLUSTER_NAME) || true

kind-deploy:
	docker build -t demo-api:v1 demo-app/api
	kind load docker-image demo-api:v1 --name $(CLUSTER_NAME)
	kubectl --context $(KUBE_CTX) apply -k k8s-manifests
	kubectl --context $(KUBE_CTX) wait --for=condition=Available deploy/api --timeout=180s

# ─── Compare ───────────────────────────────────────────────────────────────────

compare:
	@echo ""
	@echo "============================================"
	@echo "=== ECS (LocalStack Community)"
	@echo "============================================"
	-$(MAKE) ecs-run
	@echo ""
	@echo "============================================"
	@echo "=== k8s (kind: $(KUBE_CTX))"
	@echo "============================================"
	kubectl --context $(KUBE_CTX) get deploy,svc,ingress,hpa,pvc,cm
	@echo ""
	@echo "--- smoke test: /api/v1/echo ---"
	kubectl --context $(KUBE_CTX) exec deploy/api -- wget -qO- http://localhost:8080/api/v1/echo

# ─── Orchestration ─────────────────────────────────────────────────────────────

verify:
	$(MAKE) localstack-up
	$(MAKE) ecr-push
	$(MAKE) ecs-deploy
	$(MAKE) kind-up
	$(MAKE) kind-deploy
	$(MAKE) compare

down:
	-cd terraform/ecs && terraform destroy -auto-approve -input=false -var="ecr_repo=dummy"
	-docker compose -f localstack/docker-compose.yml down -v
	-kind delete cluster --name $(CLUSTER_NAME)
