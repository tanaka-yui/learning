.PHONY: up down build test compare clean logs help

help: ## ヘルプ表示
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'

up: ## 全サービス起動 (ビルド込み)
	docker compose up --build -d

down: ## 全サービス停止
	docker compose down

build: ## イメージビルドのみ
	docker compose build

test: ## キャッシュ動作テスト (HIT/MISS確認)
	@bash scripts/test-all.sh

compare: ## パフォーマンス比較
	@bash scripts/compare.sh

clean: ## キャッシュクリア (Valkey FLUSHALL + コンテナ再起動)
	docker compose exec valkey valkey-cli FLUSHALL
	docker compose restart app-cache cdn-nginx cdn-go

logs: ## 全サービスのログ表示
	docker compose logs -f

logs-%: ## 特定サービスのログ表示 (例: make logs-app-cache)
	docker compose logs -f $*
