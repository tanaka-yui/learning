.DEFAULT_GOAL := help

# Rancher Desktop で testcontainers を動かすための環境変数。
# Docker Desktop の場合は無視されても無害。
TEST_ENV := DOCKER_HOST=unix://$(HOME)/.rd/docker.sock TESTCONTAINERS_RYUK_DISABLED=true

.PHONY: help up down logs proto seed seed/catalog seed/inventory seed/user-auth test clean

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
	cd bff && go test ./...

clean: ## ボリュームも含めて削除
	docker compose down -v
