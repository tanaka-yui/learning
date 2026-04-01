.PHONY: up down reset seed psql help

help: ## このヘルプを表示
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-10s\033[0m %s\n", $$1, $$2}'

up: ## DB起動 + DDL適用
	docker compose up -d

down: ## DB停止
	docker compose down

reset: ## データ全削除 → 再起動 → シードデータ投入
	docker compose down -v
	docker compose up -d
	@echo "Waiting for PostgreSQL to be ready..."
	@until docker compose exec postgres pg_isready -U learning > /dev/null 2>&1; do sleep 1; done
	$(MAKE) seed

seed: ## テストデータ投入（Go シードツールを Docker で実行）
	docker compose run --rm seed

psql: ## chapter02 データベースに接続
	docker compose exec postgres psql -U learning -d chapter02
