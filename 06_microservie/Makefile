.DEFAULT_GOAL := help

# Rancher Desktop で testcontainers を動かすための環境変数。
# Docker Desktop の場合は無視されても無害。
TEST_ENV := DOCKER_HOST=unix://$(HOME)/.rd/docker.sock TESTCONTAINERS_RYUK_DISABLED=true

.PHONY: help up down logs proto seed seed/catalog seed/inventory seed/user-auth test clean up/flaky-20 up/flaky-60 demo/happy demo/retry demo/circuit

help: ## ヘルプを表示
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-12s\033[0m %s\n", $$1, $$2}'

up: ## 全サービス起動
	docker compose up -d --build

down: ## 停止
	docker compose down

logs: ## 全サービスのログ追従
	docker compose logs -f

proto: ## proto から Go コード再生成
	buf generate

seed: seed/catalog seed/inventory seed/user-auth ## 初期データ投入

seed/catalog:
	docker compose exec -T postgres-catalog psql -U catalog -d catalog -f - < services/catalog/seed/seed.sql

seed/inventory:
	docker compose exec -T postgres-inventory psql -U inventory -d inventory -f - < services/inventory/seed/seed.sql

seed/user-auth:
	docker compose exec -T postgres-user-auth psql -U userauth -d userauth -f - < services/user-auth/seed/seed.sql

test: ## 各 Go モジュールのユニット/インテグレーションテスト
	$(TEST_ENV) sh -c 'cd services/catalog && go test ./...'
	$(TEST_ENV) sh -c 'cd services/inventory && go test ./...'
	$(TEST_ENV) sh -c 'cd services/user-auth && go test ./...'
	$(TEST_ENV) sh -c 'cd services/payment && go test ./...'
	$(TEST_ENV) sh -c 'cd services/order && go test ./...'
	cd bff && go test ./...

clean: ## ボリュームも含めて削除
	docker compose down -v

# === Demo / 動作確認用ターゲット ===

up/flaky-20: ## FLAKE_RATE=0.2 で起動（payment が 20% 失敗）
	FLAKE_RATE=0.2 docker compose up -d --build

up/flaky-60: ## FLAKE_RATE=0.6 で起動（payment が 60% 失敗、CB Open を観察）
	FLAKE_RATE=0.6 docker compose up -d --build

demo/happy: ## ログイン→1件注文→trace を作る
	@SESSION=$$(curl -s -i -X POST http://localhost:8080/api/auth/signin \
	  -H 'Content-Type: application/json' \
	  -d '{"Email":"alice@example.com","Password":"password"}' \
	  | grep -i '^set-cookie:' | sed -E 's/.*session=([^;]+).*/\1/' | tr -d '\r\n'); \
	if [ -z "$$SESSION" ]; then echo "signin failed"; exit 1; fi; \
	curl -s -X POST http://localhost:8080/api/checkout \
	  -H "Cookie: session=$$SESSION" -H 'Content-Type: application/json' \
	  -d '{"items":[{"product_id":"p-001","quantity":1}]}' ; echo

demo/retry: ## demo/happy を10回叩く（up/flaky-20 で起動した状態を前提）
	@for i in $$(seq 1 10); do $(MAKE) demo/happy; done
	@echo "--- inspect: docker compose logs payment | tail -20 ---"

demo/circuit: ## demo/happy を20回叩く（up/flaky-60 で起動した状態を前提、CB の open を観察）
	@for i in $$(seq 1 20); do $(MAKE) demo/happy >/dev/null 2>&1 || true; done
	@echo "--- inspect order logs for breaker state ---"
	@docker compose logs order | grep -iE 'open|breaker|circuit' | tail -20 || echo "no breaker logs yet"
