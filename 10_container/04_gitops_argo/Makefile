.PHONY: up install-argocd install-flux install-workflows install-rollouts \
        install-events install-gha demo-workflow demo-event demo-canary demo-sync \
        install-aws-comparison verify down

CTX ?= kind-learning-base

up:
	kind get clusters | grep -q learning-base || \
	  kind create cluster --config ../02_kind_mesh/kind/cluster.yaml
	kubectl config use-context $(CTX)

install-argocd:
	helm repo add argo https://argoproj.github.io/argo-helm 2>/dev/null || true
	helm repo update
	helm upgrade --install argocd argo/argo-cd --version 7.7.0 \
	  -n argocd --create-namespace -f apps/argocd/install/values.yaml
	kubectl -n argocd wait --for=condition=Available deploy --all --timeout=300s
	kubectl apply -f apps/argocd/projects/demo.yaml
	kubectl apply -f apps/argocd/applicationset/demo.yaml

install-flux:
	flux install
	kubectl -n flux-system wait --for=condition=Available deploy --all --timeout=300s
	kubectl apply -f apps/flux/gitrepository.yaml
	kubectl apply -f apps/flux/kustomization-dev.yaml
	kubectl apply -f apps/flux/notification.yaml

# Task 4 finding: if the CRD install Job fails under cluster stress, pre-apply CRDs manually:
#   kubectl apply -k https://github.com/argoproj/argo-workflows/manifests/base/crds/full?ref=v3.5.10
# then retry with --set crds.install=false
install-workflows:
	helm repo add argo https://argoproj.github.io/argo-helm 2>/dev/null || true
	helm repo update
	helm upgrade --install argo-workflows argo/argo-workflows \
	  -n argo --create-namespace -f apps/workflows/install/values.yaml
	kubectl -n argo wait --for=condition=Available deploy --all --timeout=300s
	kubectl apply -f apps/workflows/workflowtemplate-shared.yaml

# Task 5 finding: Rollout readiness can take >5 min under cluster stress; timeout=300s.
install-rollouts:
	kubectl create namespace argo-rollouts 2>/dev/null || true
	kubectl apply -n argo-rollouts \
	  -f https://github.com/argoproj/argo-rollouts/releases/download/v1.7.2/install.yaml
	kubectl -n argo-rollouts wait --for=condition=Available deploy --all --timeout=300s
	kubectl create namespace demo-prod 2>/dev/null || true
	kubectl apply -f apps/rollouts/analysistemplate.yaml
	kubectl apply -f apps/rollouts/rollout-canary.yaml

# Task 6 finding: operate-workflow-sa must exist in argo-events ns and have a RoleBinding
# granting it Workflow create/update permissions in the argo ns.
install-events:
	kubectl create namespace argo-events 2>/dev/null || true
	kubectl apply -n argo-events \
	  -f https://raw.githubusercontent.com/argoproj/argo-events/v1.9.2/manifests/install.yaml
	kubectl apply -n argo-events \
	  -f https://raw.githubusercontent.com/argoproj/argo-events/v1.9.2/manifests/install-validating-webhook.yaml
	kubectl -n argo-events wait --for=condition=Available deploy --all --timeout=300s
	kubectl create serviceaccount operate-workflow-sa -n argo-events 2>/dev/null || true
	kubectl create role workflow-creator -n argo \
	  --verb=create,get,list,watch,update,patch \
	  --resource=workflows.argoproj.io \
	  2>/dev/null || true
	kubectl create rolebinding operate-workflow-sa-binding -n argo \
	  --role=workflow-creator \
	  --serviceaccount=argo-events:operate-workflow-sa \
	  2>/dev/null || true
	kubectl apply -f apps/events/eventbus.yaml
	kubectl apply -f apps/events/eventsource-webhook.yaml
	kubectl apply -f apps/events/sensor-trigger-workflow.yaml

# Task 7 finding: image.tag must NOT be set in values.yaml — chart default matches chart version.
# apps/gha-runner/install/values.yaml intentionally omits image.tag.
install-gha:
	helm upgrade --install arc \
	  oci://ghcr.io/actions/actions-runner-controller-charts/gha-runner-scale-set-controller \
	  -n arc-systems --create-namespace -f apps/gha-runner/install/values.yaml
	kubectl -n arc-systems wait --for=condition=Available deploy --all --timeout=180s
	@echo "AutoscalingRunnerSet requires GH PAT/App — see docs/07-gha-runner-controller.md"

demo-workflow:
	kubectl create -f apps/workflows/ci-build-promote.yaml
	sleep 30
	kubectl -n argo get workflow | tail

demo-event:
	kubectl -n argo-events port-forward svc/webhook-eventsource-svc 12000:12000 >/dev/null 2>&1 &
	sleep 3
	curl -X POST http://localhost:12000/example \
	  -H 'Content-Type: application/json' \
	  -d '{"trigger":"go"}'
	sleep 5
	kubectl -n argo get workflow | grep event-triggered || true
	pkill -f "port-forward svc/webhook-eventsource-svc" || true

demo-canary:
	kubectl argo rollouts -n demo-prod set image api api=localhost:5001/demo-api:v2
	kubectl argo rollouts -n demo-prod get rollout api

demo-sync:
	@echo "手動手順: envs/dev/patch-image.yaml の image タグを変更し git commit してから実行:"
	@echo "  argocd app sync demo-dev"

# Task 8 / LocalStack finding: 10-3 docker-compose.yml has SERVICES=iam,logs,cloudwatch,sts
# which lacks events,stepfunctions needed by the Terraform resources here.
# The compose YAML uses a literal value (not $${SERVICES}), so host env override has no effect.
# Manual workaround: edit ../03_ecs_vs_k8s/localstack/docker-compose.yml to add
#   events,stepfunctions to SERVICES before running this target, OR restart LocalStack
#   with: docker exec localstack localstack start --services=events,stepfunctions
# NOTE: stepfunctions/eventbridge on LocalStack Community may return 501 Not Implemented.
install-aws-comparison:
	docker compose -f ../03_ecs_vs_k8s/localstack/docker-compose.yml up -d
	cd comparison/aws/terraform && terraform init && terraform apply -auto-approve

verify:
	$(MAKE) up
	$(MAKE) install-argocd
	$(MAKE) install-workflows
	$(MAKE) install-rollouts
	$(MAKE) install-events
	$(MAKE) demo-workflow
	$(MAKE) demo-event

down:
	-kubectl delete -f apps/events/sensor-trigger-workflow.yaml
	-kubectl delete -f apps/events/eventbus.yaml
	-kubectl delete -f apps/rollouts/rollout-canary.yaml
	-helm uninstall argocd -n argocd
	-helm uninstall argo-workflows -n argo
	-helm uninstall arc -n arc-systems
	-flux uninstall --silent
	-kubectl delete namespace \
	  argocd argo argo-events argo-rollouts flux-system \
	  arc-systems demo-dev demo-stg demo-prod demo-dev-flux
